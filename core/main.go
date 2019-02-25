package core

import (
	"fmt"
	"github.com/MinterTeam/minter-explorer-extender/address"
	"github.com/MinterTeam/minter-explorer-extender/balance"
	"github.com/MinterTeam/minter-explorer-extender/block"
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

type ExtenderEnvironment struct {
	DbName      string
	DbUser      string
	DbPassword  string
	NodeApi     string
	TxChunkSize int
}

type Extender struct {
	env                 *ExtenderEnvironment
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

func NewExtender(env *ExtenderEnvironment) *Extender {

	db := pg.Connect(&pg.Options{
		User:     env.DbUser,
		Password: env.DbPassword,
		Database: env.DbName,
	})
	db.AddQueryHook(dbLogger{})

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
	coinService := coin.NewService(coinRepository, addressRepository)
	balanceService := balance.NewService(balanceRepository, nodeApi, addressRepository, coinRepository)

	return &Extender{
		env:                 env,
		nodeApi:             nodeApi,
		blockService:        block.NewBlockService(blockRepository, validatorRepository),
		eventService:        events.NewService(eventsRepository, validatorRepository, addressRepository, coinRepository),
		blockRepository:     blockRepository,
		validatorService:    validator.NewService(validatorRepository),
		transactionService:  transaction.NewService(transactionRepository, addressRepository, coinRepository, coinService),
		addressService:      address.NewService(addressRepository, balanceService.GetAddressesChannel()),
		validatorRepository: validatorRepository,
		balanceService:      balanceService,
	}
}

func (ext *Extender) Run() {
	var i, startHeight uint64

	go ext.balanceService.Run()

	lastExplorerBlock, _ := ext.blockRepository.GetLastFromDB()
	networkStatus, err := ext.nodeApi.GetStatus()
	helpers.HandleError(err)

	if lastExplorerBlock != nil {
		startHeight = lastExplorerBlock.ID + 1
		ext.blockService.SetBlockCache(lastExplorerBlock)
	} else {
		startHeight = 1
	}

	lastBlock, err := strconv.ParseUint(networkStatus.Result.LatestBlockHeight, 10, 64)
	helpers.HandleError(err)

	for i = startHeight; i <= lastBlock; i++ {
		//Pulling block data
		resp, err := ext.nodeApi.GetBlock(i)
		helpers.HandleError(err)
		ext.handleBlockResponse(resp)

		// TODO: move to gorutine
		//Pulling event data
		eventsResponse, err := ext.nodeApi.GetBlockEvents(i)
		helpers.HandleError(err)
		//Handle event data
		ext.handleEventResponse(i, eventsResponse)
	}

}

func (ext *Extender) handleBlockResponse(response *responses.BlockResponse) {
	// Save validators if not exist
	err := ext.validatorService.HandleBlockResponse(response)
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
			go ext.saveAddressesAndTransactions(height, response.Result.Time, chunk)
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

func (ext *Extender) saveAddressesAndTransactions(blockHeight uint64, blockCreatedAt time.Time, transactions []responses.Transaction) {
	// Search and save addresses from block
	err := ext.addressService.HandleTransactionsFromBlockResponse(blockHeight, transactions)
	helpers.HandleError(err)
	// Save transactions
	err = ext.transactionService.HandleTransactionsFromBlockResponse(blockHeight, blockCreatedAt, transactions)
	helpers.HandleError(err)
}
