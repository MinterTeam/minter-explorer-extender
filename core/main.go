package core

import (
	"github.com/MinterTeam/minter-explorer-extender/address"
	"github.com/MinterTeam/minter-explorer-extender/balance"
	"github.com/MinterTeam/minter-explorer-extender/block"
	"github.com/MinterTeam/minter-explorer-extender/broadcast"
	"github.com/MinterTeam/minter-explorer-extender/coin"
	"github.com/MinterTeam/minter-explorer-extender/events"
	"github.com/MinterTeam/minter-explorer-extender/transaction"
	"github.com/MinterTeam/minter-explorer-extender/validator"
	"github.com/MinterTeam/minter-explorer-tools/helpers"
	"github.com/MinterTeam/minter-explorer-tools/models"
	"github.com/daniildulin/minter-node-api"
	"github.com/daniildulin/minter-node-api/responses"
	"github.com/go-pg/pg"
	"github.com/sirupsen/logrus"
	"math"
	"os"
	"strconv"
	"time"
)

const ChasingModDiff = 2

type Extender struct {
	env                 *models.ExtenderEnvironment
	nodeApi             *minter_node_api.MinterNodeApi
	blockService        *block.Service
	addressService      *address.Service
	blockRepository     *block.Repository
	validatorService    *validator.Service
	validatorRepository *validator.Repository
	transactionService  *transaction.Service
	eventService        *events.Service
	balanceService      *balance.Service
	coinService         *coin.Service
	chasingMode         bool
	currentNodeHeight   uint64
	logger              *logrus.Entry
}

type dbLogger struct {
	logger *logrus.Entry
}

func (d dbLogger) BeforeQuery(q *pg.QueryEvent) {}

func (d dbLogger) AfterQuery(q *pg.QueryEvent) {
	d.logger.Info(q.FormattedQuery())
}

func NewExtender(env *models.ExtenderEnvironment) *Extender {
	//Init Logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetOutput(os.Stdout)
	logger.SetReportCaller(true)

	if env.Debug {
		logger.SetFormatter(&logrus.TextFormatter{
			DisableColors: false,
			FullTimestamp: true,
		})
	} else {
		logger.SetFormatter(&logrus.JSONFormatter{})
		logger.SetLevel(logrus.WarnLevel)
	}

	contextLogger := logger.WithFields(logrus.Fields{
		"version": "1.0",
		"app":     "Minter Explorer Extender",
	})

	//Init DB
	db := pg.Connect(&pg.Options{
		User:            env.DbUser,
		Password:        env.DbPassword,
		Database:        env.DbName,
		PoolSize:        20,
		MinIdleConns:    10,
		ApplicationName: env.AppName,
	})

	if env.Debug {
		db.AddQueryHook(dbLogger{logger: contextLogger})
	}

	//api
	nodeApi := minter_node_api.New(env.NodeApi)

	// Repositories
	blockRepository := block.NewRepository(db)
	validatorRepository := validator.NewRepository(db)
	transactionRepository := transaction.NewRepository(db)
	addressRepository := address.NewRepository(db)
	coinRepository := coin.NewRepository(db)
	eventsRepository := events.NewRepository(db)
	balanceRepository := balance.NewRepository(db)

	// Services
	broadcastService := broadcast.NewService(env, addressRepository, coinRepository, contextLogger)
	coinService := coin.NewService(env, nodeApi, coinRepository, addressRepository, contextLogger)
	balanceService := balance.NewService(env, balanceRepository, nodeApi, addressRepository, coinRepository, broadcastService, contextLogger)

	return &Extender{
		env:                 env,
		nodeApi:             nodeApi,
		blockService:        block.NewBlockService(blockRepository, validatorRepository, broadcastService),
		eventService:        events.NewService(env, eventsRepository, validatorRepository, addressRepository, coinRepository),
		blockRepository:     blockRepository,
		validatorService:    validator.NewService(nodeApi, validatorRepository, addressRepository, coinRepository, contextLogger),
		transactionService:  transaction.NewService(env, transactionRepository, addressRepository, validatorRepository, coinRepository, coinService, broadcastService, contextLogger),
		addressService:      address.NewService(env, addressRepository, balanceService.GetAddressesChannel(), contextLogger),
		validatorRepository: validatorRepository,
		balanceService:      balanceService,
		coinService:         coinService,
		chasingMode:         true,
		currentNodeHeight:   0,
		logger:              contextLogger,
	}
}

func (ext *Extender) Run() {
	err := ext.blockRepository.DeleteLastBlockData()
	helpers.HandleError(err)

	var height uint64

	// ----- Workers -----
	ext.runWorkers()

	lastExplorerBlock, _ := ext.blockRepository.GetLastFromDB()

	if lastExplorerBlock != nil {
		height = lastExplorerBlock.ID + 1
		ext.blockService.SetBlockCache(lastExplorerBlock)
	} else {
		height = 1
	}

	helpers.HandleError(err)

	for {
		start := time.Now()
		ext.findOutChasingMode(height)
		//Pulling block data
		blockResponse, err := ext.nodeApi.GetBlock(height)
		helpers.HandleError(err)
		if blockResponse.Error != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		//Pulling events
		eventsResponse, err := ext.nodeApi.GetBlockEvents(height)
		if err != nil {
			ext.logger.Error(err)
		}
		helpers.HandleError(err)

		ext.handleCoinsFromTransactions(blockResponse.Result.Transactions)
		ext.handleAddressesFromResponses(blockResponse, eventsResponse)
		ext.handleBlockResponse(blockResponse)

		go ext.handleEventResponse(height, eventsResponse)

		height++

		elapsed := time.Since(start)
		ext.logger.Info("Processing time: ", elapsed)
	}
}

func (ext *Extender) runWorkers() {

	// Addresses
	for w := 1; w <= ext.env.WrkSaveAddressesCount; w++ {
		go ext.addressService.SaveAddressesWorker(ext.addressService.GetSaveAddressesJobChannel())
	}

	// Transactions
	for w := 1; w <= ext.env.WrkSaveTxsCount; w++ {
		go ext.transactionService.SaveTransactionsWorker(ext.transactionService.GetSaveTxJobChannel())
	}
	for w := 1; w <= ext.env.WrkSaveTxsOutputCount; w++ {
		go ext.transactionService.SaveTransactionsOutputWorker(ext.transactionService.GetSaveTxsOutputJobChannel())
	}
	for w := 1; w <= ext.env.WrkSaveInvTxsCount; w++ {
		go ext.transactionService.SaveInvalidTransactionsWorker(ext.transactionService.GetSaveInvalidTxsJobChannel())
	}

	// Validators
	for w := 1; w <= ext.env.WrkSaveValidatorTxsCount; w++ {
		go ext.transactionService.SaveTxValidatorWorker(ext.transactionService.GetSaveTxValidatorJobChannel())
	}
	go ext.validatorService.UpdateValidatorsWorker(ext.validatorService.GetUpdateValidatorsJobChannel())
	go ext.validatorService.UpdateStakesWorker(ext.validatorService.GetUpdateStakesJobChannel())

	// Events
	for w := 1; w <= ext.env.WrkSaveRewardsCount; w++ {
		go ext.eventService.SaveRewardsWorker(ext.eventService.GetSaveRewardsJobChannel())
	}
	for w := 1; w <= ext.env.WrkSaveSlashesCount; w++ {
		go ext.eventService.SaveSlashesWorker(ext.eventService.GetSaveSlashesJobChannel())
	}

	// Balances
	go ext.balanceService.Run()
	for w := 1; w <= ext.env.WrkGetBalancesFromNodeCount; w++ {
		go ext.balanceService.GetBalancesFromNodeWorker(ext.balanceService.GetBalancesFromNodeChannel(), ext.balanceService.GetUpdateBalancesJobChannel())
	}
	for w := 1; w <= ext.env.WrkUpdateBalanceCount; w++ {
		go ext.balanceService.UpdateBalancesWorker(ext.balanceService.GetUpdateBalancesJobChannel())
	}

	//Coins
	go ext.coinService.UpdateCoinsInfoFromTxsWorker(ext.coinService.GetUpdateCoinsFromTxsJobChannel())
}

func (ext *Extender) handleAddressesFromResponses(blockResponse *responses.BlockResponse, eventsResponse *responses.EventsResponse) {
	err := ext.addressService.HandleResponses(blockResponse, eventsResponse)
	if err != nil {
		ext.logger.Error(err)
	}
	helpers.HandleError(err)
}

func (ext *Extender) handleBlockResponse(response *responses.BlockResponse) {
	// Save validators if not exist
	validators, err := ext.validatorService.HandleBlockResponse(response)
	if err != nil {
		ext.logger.Error(err)
	}
	helpers.HandleError(err)

	// Save block
	err = ext.blockService.HandleBlockResponse(response)
	if err != nil {
		ext.logger.Error(err)
	}
	helpers.HandleError(err)

	ext.linkBlockValidator(response)

	if response.Result.TxCount != "0" {
		ext.handleTransactions(response, validators)
	}

	height, err := strconv.ParseUint(response.Result.Height, 10, 64)
	if err != nil {
		ext.logger.Error(err)
	}
	helpers.HandleError(err)

	// No need to update candidate and stakes at the same time
	// Candidate will be updated in the next iteration
	if height%12 == 0 {
		ext.validatorService.GetUpdateStakesJobChannel() <- height
	} else {
		ext.validatorService.GetUpdateValidatorsJobChannel() <- height
	}
}

func (ext *Extender) handleCoinsFromTransactions(transactions []responses.Transaction) {
	if len(transactions) > 0 {
		coins, err := ext.coinService.ExtractCoinsFromTransactions(transactions)
		if err != nil {
			ext.logger.Error(err)
			helpers.HandleError(err)
		}
		if len(coins) > 0 {
			err = ext.coinService.CreateNewCoins(coins)
			if err != nil {
				ext.logger.Error(err)
				helpers.HandleError(err)
			}
		}
	}
}

func (ext *Extender) handleTransactions(response *responses.BlockResponse, validators []*models.Validator) {
	height, err := strconv.ParseUint(response.Result.Height, 10, 64)
	if err != nil {
		ext.logger.Error(err)
	}
	helpers.HandleError(err)
	chunksCount := int(math.Ceil(float64(len(response.Result.Transactions)) / float64(ext.env.TxChunkSize)))
	for i := 0; i < chunksCount; i++ {
		start := ext.env.TxChunkSize * i
		end := start + ext.env.TxChunkSize
		if end > len(response.Result.Transactions) {
			end = len(response.Result.Transactions)
		}
		ext.saveTransactions(height, response.Result.Time, response.Result.Transactions[start:end])
	}
}

func (ext *Extender) handleEventResponse(blockHeight uint64, response *responses.EventsResponse) {
	if len(response.Result.Events) > 0 {
		//Save events
		err := ext.eventService.HandleEventResponse(blockHeight, response)
		if err != nil {
			ext.logger.Error(err)
		}
		helpers.HandleError(err)
	}
}

func (ext *Extender) linkBlockValidator(response *responses.BlockResponse) {
	var links []*models.BlockValidator
	height, err := strconv.ParseUint(response.Result.Height, 10, 64)
	if err != nil {
		ext.logger.Error(err)
	}
	helpers.HandleError(err)
	for _, v := range response.Result.Validators {
		vId, err := ext.validatorRepository.FindIdByPk(helpers.RemovePrefix(v.PubKey))
		if err != nil {
			ext.logger.Error(err)
		}
		helpers.HandleError(err)
		link := models.BlockValidator{
			ValidatorID: vId,
			BlockID:     height,
			Signed:      *v.Signed,
		}
		links = append(links, &link)
	}
	err = ext.blockRepository.LinkWithValidators(links)
	if err != nil {
		ext.logger.Error(err)
	}
	helpers.HandleError(err)
}

func (ext *Extender) saveTransactions(blockHeight uint64, blockCreatedAt time.Time, transactions []responses.Transaction) {
	// Save transactions
	err := ext.transactionService.HandleTransactionsFromBlockResponse(blockHeight, blockCreatedAt, transactions)
	if err != nil {
		ext.logger.Error(err)
	}
	helpers.HandleError(err)
}

func (ext *Extender) getNodeLastBlockId() (uint64, error) {
	statusResponse, err := ext.nodeApi.GetStatus()
	if err != nil {
		ext.logger.Error(err)
		return 0, err
	}
	return strconv.ParseUint(statusResponse.Result.LatestBlockHeight, 10, 64)
}

func (ext *Extender) findOutChasingMode(height uint64) {
	var err error
	if ext.currentNodeHeight == 0 {
		ext.currentNodeHeight, err = ext.getNodeLastBlockId()
		if err != nil {
			ext.logger.Error(err)
		}
		helpers.HandleError(err)
	}
	isChasingMode := ext.currentNodeHeight-height > ChasingModDiff
	if ext.chasingMode && !isChasingMode {
		ext.currentNodeHeight, err = ext.getNodeLastBlockId()
		if err != nil {
			ext.logger.Error(err)
		}
		helpers.HandleError(err)
		ext.chasingMode = ext.currentNodeHeight-height > ChasingModDiff
	}
}
