package core

import (
	"fmt"
	"github.com/MinterTeam/minter-explorer-extender/v2/address"
	"github.com/MinterTeam/minter-explorer-extender/v2/balance"
	"github.com/MinterTeam/minter-explorer-extender/v2/block"
	"github.com/MinterTeam/minter-explorer-extender/v2/broadcast"
	"github.com/MinterTeam/minter-explorer-extender/v2/coin"
	"github.com/MinterTeam/minter-explorer-extender/v2/env"
	"github.com/MinterTeam/minter-explorer-extender/v2/events"
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/MinterTeam/minter-explorer-extender/v2/transaction"
	"github.com/MinterTeam/minter-explorer-extender/v2/validator"
	"github.com/MinterTeam/minter-explorer-tools/v4/helpers"
	"github.com/MinterTeam/minter-go-sdk/v2/api/grpc_client"
	"github.com/MinterTeam/node-grpc-gateway/api_pb"
	"github.com/go-pg/pg/v9"
	"github.com/sirupsen/logrus"
	"math"
	"os"
	"strconv"
	"time"
)

const ChasingModDiff = 2

type Extender struct {
	env                 *env.ExtenderEnvironment
	nodeApi             *grpc_client.Client
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
	currentNodeHeight   int
	logger              *logrus.Entry
}

func NewExtender(env *env.ExtenderEnvironment) *Extender {
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
		"version": "2.3.5",
		"app":     "Minter Explorer Extender",
	})

	//Init DB
	db := pg.Connect(&pg.Options{
		Addr:     fmt.Sprintf("%s:%s", env.DbHost, env.DbPort),
		User:     env.DbUser,
		Password: env.DbPassword,
		Database: env.DbName,
	})

	//api
	nodeApi, err := grpc_client.New(env.NodeApi)

	if err != nil {
		panic(err)
	}

	// Repositories
	blockRepository := block.NewRepository(db)
	validatorRepository := validator.NewRepository(db, contextLogger)
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
		eventService:        events.NewService(env, eventsRepository, validatorRepository, addressRepository, coinRepository, coinService, balanceRepository, contextLogger),
		blockRepository:     blockRepository,
		validatorService:    validator.NewService(env, nodeApi, validatorRepository, addressRepository, coinRepository, contextLogger),
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
	//check connections to node
	_, err := ext.nodeApi.Status()
	if err == nil {
		err = ext.blockRepository.DeleteLastBlockData()
	} else {
		ext.logger.Error(err)
	}
	helpers.HandleError(err)

	var height int

	// ----- Workers -----
	ext.runWorkers()

	lastExplorerBlock, _ := ext.blockRepository.GetLastFromDB()

	if lastExplorerBlock != nil {
		height = int(lastExplorerBlock.ID) + 1
		ext.blockService.SetBlockCache(lastExplorerBlock)
	} else {
		height = 1
	}

	for {
		start := time.Now()
		ext.findOutChasingMode(height)
		//Pulling block data
		blockResponse, err := ext.nodeApi.Block(height)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		//Pulling events
		eventsResponse, err := ext.nodeApi.Events(height)
		if err != nil {
			ext.logger.Panic(err)
		}

		ext.handleCoinsFromTransactions(blockResponse.Transactions)
		ext.handleAddressesFromResponses(blockResponse, eventsResponse)
		ext.handleBlockResponse(blockResponse)

		if height%ext.env.RewardAggregateEveryBlocksCount == 0 {
			go ext.eventService.AggregateRewards(ext.env.RewardAggregateTimeInterval, uint64(height))
		}
		go ext.handleEventResponse(uint64(height), eventsResponse)

		ext.logger.Info("Block: ", height)
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
	go ext.transactionService.UpdateTxsIndexWorker()

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
	go ext.coinService.UpdateCoinsInfoFromCoinsMap(ext.coinService.GetUpdateCoinsFromCoinsMapJobChannel())
}

func (ext *Extender) handleAddressesFromResponses(blockResponse *api_pb.BlockResponse, eventsResponse *api_pb.EventsResponse) {
	err := ext.addressService.HandleResponses(blockResponse, eventsResponse)
	if err != nil {
		ext.logger.Panic(err)
	}
}

func (ext *Extender) handleBlockResponse(response *api_pb.BlockResponse) {
	// Save validators if not exist
	err := ext.validatorService.HandleBlockResponse(response)
	if err != nil {
		ext.logger.Panic(err)
	}

	// Save block
	err = ext.blockService.HandleBlockResponse(response)
	if err != nil {
		ext.logger.Panic(err)
	}

	ext.linkBlockValidator(response)

	//first block don't have validators
	if response.TransactionCount != "0" {
		ext.handleTransactions(response)
	}

	height, err := strconv.ParseUint(response.Height, 10, 64)
	if err != nil {
		ext.logger.Panic(err)
	}

	// No need to update candidate and stakes at the same time
	// Candidate will be updated in the next iteration
	if height%120 == 0 {
		ext.validatorService.GetUpdateStakesJobChannel() <- int(height)
		ext.validatorService.GetUpdateValidatorsJobChannel() <- int(height)
	}
}

func (ext *Extender) handleCoinsFromTransactions(transactions []*api_pb.BlockResponse_Transaction) {
	if len(transactions) > 0 {
		coins, err := ext.coinService.ExtractCoinsFromTransactions(transactions)
		if err != nil {
			ext.logger.Fatal(err)
		}
		if len(coins) > 0 {
			err = ext.coinService.CreateNewCoins(coins)
			if err != nil {
				ext.logger.Fatal(err)
			}
		}
	}
}

func (ext *Extender) handleTransactions(response *api_pb.BlockResponse) {
	height, err := strconv.ParseUint(response.Height, 10, 64)
	if err != nil {
		ext.logger.Panic(err)
	}
	chunksCount := int(math.Ceil(float64(len(response.Transactions)) / float64(ext.env.TxChunkSize)))
	for i := 0; i < chunksCount; i++ {
		start := ext.env.TxChunkSize * i
		end := start + ext.env.TxChunkSize
		if end > len(response.Transactions) {
			end = len(response.Transactions)
		}

		layout := "2006-01-02T15:04:05Z"
		blockTime, err := time.Parse(layout, response.Time)
		if err != nil {
			ext.logger.Panic(err)
		}

		ext.saveTransactions(height, blockTime, response.Transactions[start:end])
	}
}

func (ext *Extender) handleEventResponse(blockHeight uint64, response *api_pb.EventsResponse) {
	if len(response.Events) > 0 {
		//Save events
		err := ext.eventService.HandleEventResponse(blockHeight, response.Events)
		if err != nil {
			ext.logger.Fatal(err)
		}
	}
}

func (ext *Extender) linkBlockValidator(response *api_pb.BlockResponse) {
	if response.Height == "1" {
		return
	}
	var links []*models.BlockValidator
	height, err := strconv.ParseUint(response.Height, 10, 64)
	if err != nil {
		ext.logger.Error(err)
	}
	helpers.HandleError(err)
	for _, v := range response.Validators {
		vId, err := ext.validatorRepository.FindIdByPk(helpers.RemovePrefix(v.PublicKey))
		if err != nil {
			ext.logger.Error(err)
		}
		helpers.HandleError(err)
		link := models.BlockValidator{
			ValidatorID: uint64(vId),
			BlockID:     height,
			Signed:      v.Signed,
		}
		links = append(links, &link)
	}
	err = ext.blockRepository.LinkWithValidators(links)
	if err != nil {
		ext.logger.Error(err)
	}
	helpers.HandleError(err)
}

func (ext *Extender) saveTransactions(blockHeight uint64, blockCreatedAt time.Time, transactions []*api_pb.BlockResponse_Transaction) {
	// Save transactions
	err := ext.transactionService.HandleTransactionsFromBlockResponse(blockHeight, blockCreatedAt, transactions)
	if err != nil {
		ext.logger.Panic(err)
	}
}

func (ext *Extender) getNodeLastBlockId() (int, error) {
	statusResponse, err := ext.nodeApi.Status()
	if err != nil {
		ext.logger.Error(err)
		return 0, err
	}
	height, err := strconv.ParseInt(statusResponse.LatestBlockHeight, 10, 64)
	if err != nil {
		ext.logger.Error(err)
		return 0, err
	}
	return int(height), err
}

func (ext *Extender) findOutChasingMode(height int) {
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
