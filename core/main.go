package core

import (
	"context"
	"crypto/tls"
	"fmt"
	genesisUploader "github.com/MinterTeam/explorer-genesis-uploader/core"
	genesisEnv "github.com/MinterTeam/explorer-genesis-uploader/env"
	"github.com/MinterTeam/minter-explorer-api/v2/coins"
	"github.com/MinterTeam/minter-explorer-extender/v2/address"
	"github.com/MinterTeam/minter-explorer-extender/v2/balance"
	"github.com/MinterTeam/minter-explorer-extender/v2/block"
	"github.com/MinterTeam/minter-explorer-extender/v2/broadcast"
	"github.com/MinterTeam/minter-explorer-extender/v2/coin"
	"github.com/MinterTeam/minter-explorer-extender/v2/env"
	"github.com/MinterTeam/minter-explorer-extender/v2/events"
	"github.com/MinterTeam/minter-explorer-extender/v2/liquidity_pool"
	"github.com/MinterTeam/minter-explorer-extender/v2/metrics"
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/MinterTeam/minter-explorer-extender/v2/orderbook"
	"github.com/MinterTeam/minter-explorer-extender/v2/transaction"
	"github.com/MinterTeam/minter-explorer-extender/v2/validator"
	"github.com/MinterTeam/minter-explorer-tools/v4/helpers"
	"github.com/MinterTeam/minter-go-sdk/v2/api/grpc_client"
	"github.com/MinterTeam/node-grpc-gateway/api_pb"
	"github.com/go-pg/pg/v10"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/status"
	"math"
	"os"
	"regexp"
	"strings"
	"time"
)

const ChasingModDiff = 121

var Version string

type Extender struct {
	Metrics              *metrics.Metrics
	env                  *env.ExtenderEnvironment
	nodeApi              *grpc_client.Client
	blockService         *block.Service
	addressService       *address.Service
	blockRepository      *block.Repository
	validatorService     *validator.Service
	validatorRepository  *validator.Repository
	transactionService   *transaction.Service
	eventService         *events.Service
	balanceService       *balance.Service
	coinService          *coin.Service
	broadcastService     *broadcast.Service
	orderBookService     *orderbook.Service
	chasingMode          bool
	startBlockHeight     uint64
	currentNodeHeight    uint64
	lastLPSnapshotHeight uint64
	log                  *logrus.Entry
	lpSnapshotChannel    chan *api_pb.BlockResponse
	lpWorkerChannel      chan *api_pb.BlockResponse
	orderBookChannel     chan *api_pb.BlockResponse
}

type ExtenderElapsedTime struct {
	Height                       uint64
	GettingBlock                 time.Duration
	GettingEvents                time.Duration
	HandleCoinsFromTransactions  time.Duration
	HandleAddressesFromResponses time.Duration
	HandleBlockResponse          time.Duration
	Total                        time.Duration
}

type eventHook struct {
	beforeTime time.Time
	log        *logrus.Logger
}

func (eh eventHook) BeforeQuery(ctx context.Context, event *pg.QueryEvent) (context.Context, error) {
	if event.Stash == nil {
		event.Stash = make(map[interface{}]interface{})
	}
	event.Stash["query_time"] = time.Now()
	return ctx, nil
}
func (eh eventHook) AfterQuery(ctx context.Context, event *pg.QueryEvent) error {
	critical := time.Millisecond * 500
	result := time.Duration(0)
	if event.Stash != nil {
		if v, ok := event.Stash["query_time"]; ok {
			result = time.Now().Sub(v.(time.Time))
		}
	}

	if result > critical {
		bigQueryLog, err := os.OpenFile("big_query.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
		if err != nil {
			eh.log.Error("error opening file: %v", err)
		}
		// don't forget to close it
		defer bigQueryLog.Close()
		eh.log.SetReportCaller(false)
		eh.log.SetFormatter(&logrus.JSONFormatter{})
		eh.log.SetOutput(bigQueryLog)
		q, err := event.UnformattedQuery()
		if err != nil {
			eh.log.Error(err)
		}

		r := regexp.MustCompile("\\s+")
		replace := r.ReplaceAllString(fmt.Sprintf("%v", string(q)), " ")

		eh.log.WithFields(logrus.Fields{
			"query": strings.TrimSpace(replace),
			"time":  fmt.Sprintf("%s", result),
		}).Error("DB query time is too height")
	}
	return nil
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
		"version": Version,
		"app":     "Minter Explorer Extender",
	})

	//Init DB
	pgOptions := &pg.Options{
		Addr:     fmt.Sprintf("%s:%s", env.DbHost, env.DbPort),
		User:     env.DbUser,
		Password: env.DbPassword,
		Database: env.DbName,
	}
	if os.Getenv("POSTGRES_SSL_ENABLED") == "true" {
		pgOptions.TLSConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	//hookImpl := eventHook{
	//	log:        logrus.New(),
	//	beforeTime: time.Now(),
	//}

	db := pg.Connect(pgOptions)
	//db.AddQueryHook(hookImpl)

	uploader := genesisUploader.New(genesisEnv.Config{
		Debug:              false,
		PostgresHost:       env.DbHost,
		PostgresPort:       env.DbPort,
		PostgresDB:         env.DbName,
		PostgresUser:       env.DbUser,
		PostgresPassword:   env.DbPassword,
		PostgresSSLEnabled: os.Getenv("POSTGRES_SSL_ENABLED") == "true",
		MinterBaseCoin:     env.BaseCoin,
		NodeGrpc:           env.NodeApi,
		AddressChunkSize:   uint64(env.AddrChunkSize),
		CoinsChunkSize:     1000,
		BalanceChunkSize:   10000,
		StakeChunkSize:     uint64(env.StakeChunkSize),
		ValidatorChunkSize: uint64(env.StakeChunkSize),
	})
	err := uploader.Do()
	if err != nil {
		logger.Warn(err)
	}

	//api
	nodeApi, err := grpc_client.New(env.NodeApi)
	if err != nil {
		panic(err)
	}

	nodeStatus, err := nodeApi.Status()
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

	liquidityPoolRepository := liquidity_pool.NewRepository(db)

	orderbookRepository := orderbook.NewRepository(db)

	coins.GlobalRepository = coins.NewRepository(db) //temporary solution

	// Services
	addressService := address.NewService(env, addressRepository, contextLogger)
	broadcastService := broadcast.NewService(env, addressRepository, coinRepository, nodeApi, contextLogger)
	balanceService := balance.NewService(env, balanceRepository, nodeApi, addressService, coinRepository, broadcastService, contextLogger)
	coinService := coin.NewService(env, nodeApi, coinRepository, addressRepository, contextLogger)
	validatorService := validator.NewService(env, nodeApi, validatorRepository, addressRepository, coinRepository, contextLogger)
	eventService := events.NewService(env, eventsRepository, validatorRepository, addressRepository, coinRepository, coinService, blockRepository, orderbookRepository, balanceRepository, broadcastService, contextLogger, nodeStatus.InitialHeight+1)
	orderBookService := orderbook.NewService(db, addressRepository, liquidityPoolRepository, contextLogger)

	return &Extender{
		Metrics:             metrics.New(),
		env:                 env,
		nodeApi:             nodeApi,
		blockService:        block.NewBlockService(blockRepository, validatorRepository, broadcastService),
		eventService:        eventService,
		blockRepository:     blockRepository,
		validatorService:    validatorService,
		transactionService:  transaction.NewService(env, transactionRepository, addressRepository, validatorRepository, coinRepository, coinService, broadcastService, contextLogger, validatorService.GetUnbondSaverJobChannel(), liquidityPoolRepository, validatorService.GetMoveStakeJobChannel()),
		addressService:      addressService,
		validatorRepository: validatorRepository,
		balanceService:      balanceService,
		coinService:         coinService,
		broadcastService:    broadcastService,
		orderBookService:    orderBookService,
		chasingMode:         false,
		currentNodeHeight:   0,
		startBlockHeight:    nodeStatus.InitialHeight + 1,
		log:                 contextLogger,
		lpSnapshotChannel:   make(chan *api_pb.BlockResponse),
		lpWorkerChannel:     make(chan *api_pb.BlockResponse),
		orderBookChannel:    make(chan *api_pb.BlockResponse),
	}
}

func (ext *Extender) GetInfo() {
	fmt.Printf("%s v%s\n", "Minter Explorer Extender", Version)
}

func (ext *Extender) Run() {
	//check connections to node
	_, err := ext.nodeApi.Status()
	if err == nil {
		err = ext.blockRepository.DeleteLastBlockData()
	}
	if err != nil {
		ext.log.Fatal(err)
	}

	var height uint64

	// ----- Workers -----
	ext.runWorkers()

	lastExplorerBlock, err := ext.blockRepository.GetLastFromDB()
	if err != nil && err != pg.ErrNoRows {
		ext.log.Fatal(err)
	}

	if lastExplorerBlock != nil {
		height = lastExplorerBlock.ID + 1
		ext.blockService.SetBlockCache(lastExplorerBlock)
	} else {
		height = ext.startBlockHeight
	}

	for {
		eet := ExtenderElapsedTime{
			Height:                       height,
			GettingBlock:                 0,
			GettingEvents:                0,
			HandleCoinsFromTransactions:  0,
			HandleAddressesFromResponses: 0,
			HandleBlockResponse:          0,
			Total:                        0,
		}

		start := time.Now()
		//ext.findOutChasingMode(height)

		//Pulling block data
		countStart := time.Now()
		blockResponse, err := ext.nodeApi.BlockExtended(height, true, true)
		if err != nil {
			grpcErr, ok := status.FromError(err)
			if !ok {
				ext.log.Error(err)
				time.Sleep(2 * time.Second)
				continue
			}
			if grpcErr.Message() == "Block not found" || grpcErr.Message() == "Block results not found" {
				time.Sleep(2 * time.Second)
				continue
			}
			ext.log.Fatal(err)
		}

		eet.GettingBlock = time.Since(countStart)

		countStart = time.Now()
		ext.handleCoinsFromTransactions(blockResponse)
		eet.HandleCoinsFromTransactions = time.Since(countStart)

		countStart = time.Now()
		ext.handleAddressesFromResponses(blockResponse)
		eet.HandleAddressesFromResponses = time.Since(countStart)

		countStart = time.Now()
		ext.handleBlockResponse(blockResponse)
		eet.HandleBlockResponse = time.Since(countStart)

		ext.balanceService.UpdateChannel() <- blockResponse

		go ext.handleEventResponse(height, blockResponse)

		if len(blockResponse.Transactions) > 0 {
			ext.orderBookChannel <- blockResponse
		}

		//ext.validatorService.GetUpdateStakesJobChannel() <- height
		ext.validatorService.GetUpdateValidatorsJobChannel() <- height
		ext.validatorService.GetClearJobChannel() <- height

		eet.Total = time.Since(start)
		ext.printSpentTimeLog(eet)

		height++
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

	//go ext.validatorService.UpdateStakesWorker(ext.validatorService.GetUpdateStakesJobChannel())

	// Events
	for w := 1; w <= ext.env.WrkSaveRewardsCount; w++ {
		go ext.eventService.SaveRewardsWorker(ext.eventService.GetSaveRewardsJobChannel())
	}
	for w := 1; w <= ext.env.WrkSaveSlashesCount; w++ {
		go ext.eventService.SaveSlashesWorker(ext.eventService.GetSaveSlashesJobChannel())
	}

	// Balances
	go ext.balanceService.BalanceManager()

	//Coins
	go ext.coinService.UpdateCoinsInfoFromTxsWorker(ext.coinService.GetUpdateCoinsFromTxsJobChannel())
	go ext.coinService.UpdateCoinsInfoFromCoinsMap(ext.coinService.GetUpdateCoinsFromCoinsMapJobChannel())
	go ext.coinService.UpdateHubInfoWorker()

	//Unbonds
	go ext.validatorService.UnbondSaverWorker(ext.validatorService.GetUnbondSaverJobChannel())
	//Move Stake
	go ext.validatorService.MoveStakeWorker(ext.validatorService.GetMoveStakeJobChannel())

	go ext.validatorService.ClearMoveStakeAndUnbondWorker(ext.validatorService.GetClearJobChannel())

	//OrderBook
	go ext.orderBookService.OrderBookWorker(ext.orderBookChannel)
	go ext.orderBookService.UpdateOrderBookWorker(ext.orderBookService.UpdateOrderChannel())

	//Broadcast
	go ext.broadcastService.Manager()
}

func (ext *Extender) handleAddressesFromResponses(blockResponse *api_pb.BlockResponse) {
	err := ext.addressService.SaveAddressesFromResponses(blockResponse)
	if err != nil {
		ext.log.Panic(err)
	}
}

func (ext *Extender) handleBlockResponse(response *api_pb.BlockResponse) {
	// Save validators if not exist
	err := ext.validatorService.HandleBlockResponse(response)
	if err != nil {
		ext.log.Panic(err)
	}

	// Save block
	err = ext.blockService.HandleBlockResponse(response)
	if err != nil {
		ext.log.Panic(err)
	}

	ext.linkBlockValidator(response)

	//first block don't have validators
	if response.TransactionCount > 0 {
		ext.handleTransactions(response)
	}

}

func (ext *Extender) handleCoinsFromTransactions(block *api_pb.BlockResponse) {
	if len(block.Transactions) == 0 {
		return
	}
	err := ext.coinService.HandleCoinsFromBlock(block)
	if err != nil {
		ext.log.Fatal(err)
	}
}

func (ext *Extender) handleTransactions(response *api_pb.BlockResponse) {
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
			ext.log.Panic(err)
		}

		ext.saveTransactions(response.Height, blockTime, response.Transactions[start:end])
	}
}

func (ext *Extender) handleEventResponse(blockHeight uint64, response *api_pb.BlockResponse) {
	if len(response.Events) > 0 {
		//Save events
		err := ext.eventService.HandleEventResponse(blockHeight, response)
		if err != nil {
			ext.log.Fatal(err)
		}
	}
}

func (ext *Extender) linkBlockValidator(response *api_pb.BlockResponse) {
	if response.Height == 1 {
		return
	}
	var links []*models.BlockValidator
	for _, v := range response.Validators {
		vId, err := ext.validatorRepository.FindIdByPk(helpers.RemovePrefix(v.PublicKey))
		if err != nil {
			ext.log.Error(err)
		}
		helpers.HandleError(err)
		link := models.BlockValidator{
			ValidatorID: uint64(vId),
			BlockID:     response.Height,
			Signed:      v.Signed,
		}
		links = append(links, &link)
	}
	err := ext.blockRepository.LinkWithValidators(links)
	if err != nil {
		ext.log.Fatal(err)
	}
}

func (ext *Extender) saveTransactions(blockHeight uint64, blockCreatedAt time.Time, transactions []*api_pb.TransactionResponse) {
	// Save transactions
	err := ext.transactionService.HandleTransactionsFromBlockResponse(blockHeight, blockCreatedAt, transactions)
	if err != nil {
		ext.log.Panic(err)
	}
}

func (ext *Extender) getNodeLastBlockId() (uint64, error) {
	statusResponse, err := ext.nodeApi.Status()
	if err != nil {
		ext.log.Error(err)
		return 0, err
	}
	return statusResponse.LatestBlockHeight, err
}

func (ext *Extender) findOutChasingMode(height uint64) {
	var err error

	if ext.currentNodeHeight == 0 {
		ext.currentNodeHeight, err = ext.getNodeLastBlockId()
		if err != nil {
			ext.log.Fatal(err)
		}
	}

	isChasingMode := ext.currentNodeHeight-height > ChasingModDiff
	if ext.chasingMode && !isChasingMode {
		ext.currentNodeHeight, err = ext.getNodeLastBlockId()
		if err != nil {
			ext.log.Fatal(err)
		}
		ext.chasingMode = ext.currentNodeHeight-height > ChasingModDiff
	}

	ext.broadcastService.SetChasingMode(ext.chasingMode)
	ext.balanceService.SetChasingMode(ext.chasingMode)
	//ext.liquidityPoolService.SetChasingMode(ext.chasingMode)
}

func (ext *Extender) printSpentTimeLog(eet ExtenderElapsedTime) {

	critical := 7 * time.Second

	if eet.Total > critical {
		ext.log.WithFields(logrus.Fields{
			"getting block time":  eet.GettingBlock,
			"getting events time": eet.GettingEvents,
			"handle addresses":    eet.HandleAddressesFromResponses,
			"handle coins":        eet.HandleCoinsFromTransactions,
			"handle block":        eet.HandleBlockResponse,
			"block":               eet.Height,
			"time":                fmt.Sprintf("%s", eet.Total),
		}).Warning("Processing time is too height")
	}

	ext.log.WithFields(logrus.Fields{
		"getting block time":  eet.GettingBlock,
		"getting events time": eet.GettingEvents,
		"handle addresses":    eet.HandleAddressesFromResponses,
		"handle coins":        eet.HandleCoinsFromTransactions,
		"handle block":        eet.HandleBlockResponse,
	}).Info(fmt.Sprintf("Block: %d Processing time: %s", eet.Height, eet.Total))
}
