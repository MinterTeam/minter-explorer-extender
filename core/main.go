package core

import (
	"fmt"
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
	"log"
	"math"
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
	chasingMode         bool
	currentNodeHeight   uint64
}

type dbLogger struct{}

func (d dbLogger) BeforeQuery(q *pg.QueryEvent) {}

func (d dbLogger) AfterQuery(q *pg.QueryEvent) {
	fmt.Println(q.FormattedQuery())
}

func NewExtender(env *models.ExtenderEnvironment) *Extender {

	db := pg.Connect(&pg.Options{
		User:            env.DbUser,
		Password:        env.DbPassword,
		Database:        env.DbName,
		PoolSize:        20,
		MinIdleConns:    10,
		ApplicationName: env.AppName,
	})

	if env.Debug {
		db.AddQueryHook(dbLogger{})
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
	broadcastService := broadcast.NewService(env, addressRepository, coinRepository)
	coinService := coin.NewService(coinRepository, addressRepository)
	balanceService := balance.NewService(env, balanceRepository, nodeApi, addressRepository, coinRepository, broadcastService)

	return &Extender{
		env:                 env,
		nodeApi:             nodeApi,
		blockService:        block.NewBlockService(blockRepository, validatorRepository, broadcastService),
		eventService:        events.NewService(env, eventsRepository, validatorRepository, addressRepository, coinRepository),
		blockRepository:     blockRepository,
		validatorService:    validator.NewService(nodeApi, validatorRepository, addressRepository, coinRepository),
		transactionService:  transaction.NewService(env, transactionRepository, addressRepository, validatorRepository, coinRepository, coinService, broadcastService),
		addressService:      address.NewService(env, addressRepository, balanceService.GetAddressesChannel()),
		validatorRepository: validatorRepository,
		balanceService:      balanceService,
		chasingMode:         true,
		currentNodeHeight:   0,
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
		helpers.HandleError(err)

		ext.handleAddressesFromResponses(blockResponse, eventsResponse)
		ext.handleBlockResponse(blockResponse)

		go ext.handleEventResponse(height, eventsResponse)

		height++

		elapsed := time.Since(start)
		log.Printf("Processing time %s", elapsed)
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
	go ext.validatorService.UpdateValidatorsAndStakesWorker(ext.validatorService.GetUpdateValidatorsAndStakesJobChannel())

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
}

func (ext *Extender) handleAddressesFromResponses(blockResponse *responses.BlockResponse, eventsResponse *responses.EventsResponse) {
	err := ext.addressService.HandleResponses(blockResponse, eventsResponse)
	helpers.HandleError(err)
}

func (ext *Extender) handleBlockResponse(response *responses.BlockResponse) {
	// Save validators if not exist
	validators, err := ext.validatorService.HandleBlockResponse(response)
	helpers.HandleError(err)

	// Save block
	err = ext.blockService.HandleBlockResponse(response)
	helpers.HandleError(err)

	ext.linkBlockValidator(response)

	if response.Result.TxCount != "0" {
		ext.handleTransactions(response, validators)
	}

	height, err := strconv.ParseUint(response.Result.Height, 10, 64)
	helpers.HandleError(err)

	if !ext.chasingMode && height%2 == 0 {
		ext.validatorService.GetUpdateValidatorsAndStakesJobChannel() <- models.BlockValidators{Height: height, Validators: validators}
	}
}

func (ext *Extender) handleTransactions(response *responses.BlockResponse, validators []*models.Validator) {
	height, err := strconv.ParseUint(response.Result.Height, 10, 64)
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
		helpers.HandleError(err)
	}
}

func (ext *Extender) linkBlockValidator(response *responses.BlockResponse) {
	var links []*models.BlockValidator
	height, err := strconv.ParseUint(response.Result.Height, 10, 64)
	helpers.HandleError(err)
	for _, v := range response.Result.Validators {
		vId, err := ext.validatorRepository.FindIdByPk(helpers.RemovePrefix(v.PubKey))
		helpers.HandleError(err)
		link := models.BlockValidator{
			ValidatorID: vId,
			BlockID:     height,
			Signed:      v.Signed,
		}
		links = append(links, &link)
	}
	err = ext.blockRepository.LinkWithValidators(links)
	helpers.HandleError(err)
}

func (ext *Extender) saveTransactions(blockHeight uint64, blockCreatedAt time.Time, transactions []responses.Transaction) {
	// Save transactions
	err := ext.transactionService.HandleTransactionsFromBlockResponse(blockHeight, blockCreatedAt, transactions)
	helpers.HandleError(err)
}

func (ext *Extender) getNodeLastBlockId() (uint64, error) {
	statusResponse, err := ext.nodeApi.GetStatus()
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(statusResponse.Result.LatestBlockHeight, 10, 64)
}

func (ext *Extender) findOutChasingMode(height uint64) {
	var err error
	if ext.currentNodeHeight == 0 {
		ext.currentNodeHeight, err = ext.getNodeLastBlockId()
		helpers.HandleError(err)
	}
	isChasingMode := ext.currentNodeHeight-height > ChasingModDiff
	if ext.chasingMode && !isChasingMode {
		ext.currentNodeHeight, err = ext.getNodeLastBlockId()
		helpers.HandleError(err)
		ext.chasingMode = ext.currentNodeHeight-height > ChasingModDiff
	}
}
