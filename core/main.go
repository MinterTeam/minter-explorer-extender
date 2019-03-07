package core

import (
	"fmt"
	"github.com/MinterTeam/minter-explorer-extender/address"
	"github.com/MinterTeam/minter-explorer-extender/balance"
	"github.com/MinterTeam/minter-explorer-extender/block"
	"github.com/MinterTeam/minter-explorer-extender/broadcast"
	"github.com/MinterTeam/minter-explorer-extender/coin"
	"github.com/MinterTeam/minter-explorer-extender/events"
	"github.com/MinterTeam/minter-explorer-extender/helpers"
	"github.com/MinterTeam/minter-explorer-extender/models"
	"github.com/MinterTeam/minter-explorer-extender/transaction"
	"github.com/MinterTeam/minter-explorer-extender/validator"
	"github.com/daniildulin/minter-node-api"
	"github.com/daniildulin/minter-node-api/responses"
	"github.com/go-pg/pg"
	"math"
	"strconv"
	"time"
)

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
		ApplicationName: "Minter Extender",
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
	broadcastService := broadcast.NewService(env, addressRepository)
	coinService := coin.NewService(coinRepository, addressRepository)
	balanceService := balance.NewService(balanceRepository, nodeApi, addressRepository, coinRepository, broadcastService)

	return &Extender{
		env:                 env,
		nodeApi:             nodeApi,
		blockService:        block.NewBlockService(blockRepository, validatorRepository, broadcastService),
		eventService:        events.NewService(eventsRepository, validatorRepository, addressRepository, coinRepository),
		blockRepository:     blockRepository,
		validatorService:    validator.NewService(validatorRepository, addressRepository, coinRepository),
		transactionService:  transaction.NewService(transactionRepository, addressRepository, validatorRepository, coinRepository, coinService, broadcastService),
		addressService:      address.NewService(addressRepository, balanceService.GetAddressesChannel()),
		validatorRepository: validatorRepository,
		balanceService:      balanceService,
	}
}

func (ext *Extender) Run() {
	go ext.balanceService.Run()

	err := ext.blockRepository.DeleteLastBlockData()
	helpers.HandleError(err)

	var startHeight uint64
	lastExplorerBlock, _ := ext.blockRepository.GetLastFromDB()

	if lastExplorerBlock != nil {
		startHeight = lastExplorerBlock.ID + 1
		ext.blockService.SetBlockCache(lastExplorerBlock)
	} else {
		startHeight = 1
	}

	for {
		//Pulling block data
		resp, err := ext.nodeApi.GetBlock(startHeight)
		helpers.HandleError(err)
		if resp.Error != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		ext.handleBlockResponse(resp)

		// TODO: move to gorutine
		//Pulling event data
		eventsResponse, err := ext.nodeApi.GetBlockEvents(startHeight)
		helpers.HandleError(err)
		//Handle event data
		ext.handleEventResponse(startHeight, eventsResponse)

		startHeight++
	}

}

func (ext *Extender) handleBlockResponse(response *responses.BlockResponse) {
	// Save validators if not exist
	validators, err := ext.validatorService.HandleBlockResponse(response)
	helpers.HandleError(err)
	// Save block
	err = ext.blockService.HandleBlockResponse(response)
	helpers.HandleError(err)
	err = ext.linkBlockValidator(response)
	helpers.HandleError(err)

	if response.Result.TxCount != "0" {
		height, err := strconv.ParseUint(response.Result.Height, 10, 64)
		helpers.HandleError(err)
		chunksCount := int(math.Ceil(float64(len(response.Result.Transactions)) / float64(ext.env.TxChunkSize)))
		chunks := make([][]responses.Transaction, chunksCount)
		for i := 0; i < chunksCount; i++ {
			start := ext.env.TxChunkSize * i
			end := start + ext.env.TxChunkSize
			if end > len(response.Result.Transactions) {
				end = len(response.Result.Transactions)
			}
			chunks[i] = response.Result.Transactions[start:end]
		}
		for _, chunk := range chunks {
			ext.saveAddressesAndTransactions(height, response.Result.Time, chunk, validators)
		}
	}
}

func (ext *Extender) handleEventResponse(blockHeight uint64, response *responses.EventsResponse) {
	if len(response.Result.Events) > 0 {
		//TODO: split
		//Search and save addresses from block
		err := ext.addressService.HandleEventsResponse(blockHeight, response)
		helpers.HandleError(err)
		//Save events
		err = ext.eventService.HandleEventResponse(blockHeight, response)
		helpers.HandleError(err)
	}
}

func (ext *Extender) linkBlockValidator(response *responses.BlockResponse) error {
	var links []*models.BlockValidator
	height, err := strconv.ParseUint(response.Result.Height, 10, 64)
	if err != nil {
		return err
	}
	for _, v := range response.Result.Validators {
		vId, err := ext.validatorRepository.FindIdByPk(
			helpers.RemovePrefix(v.PubKey))
		if err != nil {
			return err
		}
		links = append(links, &models.BlockValidator{
			ValidatorID: vId,
			BlockID:     height,
			Signed:      v.Signed,
		})
	}
	err = ext.blockRepository.LinkWithValidators(links)
	if err != nil {
		return err
	}
	return nil
}

func (ext *Extender) saveAddressesAndTransactions(blockHeight uint64, blockCreatedAt time.Time, transactions []responses.Transaction, validators []*models.Validator) {
	// Search and save addresses from block
	err := ext.addressService.HandleTransactionsFromBlockResponse(blockHeight, transactions)
	helpers.HandleError(err)
	// Save transactions
	err = ext.transactionService.HandleTransactionsFromBlockResponse(blockHeight, blockCreatedAt, transactions, validators)
	helpers.HandleError(err)

	//TODO: temporary here
	// have to be handled after addresses
	go ext.updateValidatorsData(validators, blockHeight)
}

func (ext Extender) updateValidatorsData(validators []*models.Validator, blockHeight uint64) error {
	for _, vlr := range validators {
		resp, err := ext.nodeApi.GetCandidate(vlr.GetPublicKey(), blockHeight)
		if err != nil {
			return err
		}
		err = ext.validatorService.UpdateValidatorsInfoAndStakes(resp)
		if err != nil {
			return err
		}
	}
	return nil
}
