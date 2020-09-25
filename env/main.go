package env

import (
	"github.com/sirupsen/logrus"
	"os"
	"strconv"
)

type ExtenderEnvironment struct {
	BaseCoin                        string
	CoinsUpdateTime                 int
	Debug                           bool
	DbHost                          string
	DbPort                          string
	DbName                          string
	DbUser                          string
	DbPassword                      string
	WsLink                          string
	WsKey                           string
	NodeApi                         string
	ApiPort                         int
	TxChunkSize                     int
	AddrChunkSize                   int
	EventsChunkSize                 int
	StakeChunkSize                  int
	WrkSaveRewardsCount             int
	WrkSaveSlashesCount             int
	WrkSaveTxsCount                 int
	WrkSaveTxsOutputCount           int
	WrkSaveInvTxsCount              int
	WrkSaveAddressesCount           int
	WrkSaveValidatorTxsCount        int
	WrkUpdateBalanceCount           int
	WrkGetBalancesFromNodeCount     int
	WrkUpdateTxsIndexNumBlocks      int
	WrkUpdateTxsIndexTime           int
	RewardAggregateEveryBlocksCount uint64
	RewardAggregateTimeInterval     string
}

func New() *ExtenderEnvironment {

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetOutput(os.Stdout)
	logger.SetReportCaller(true)

	txChunkSize, err := strconv.ParseInt(os.Getenv("APP_TX_CHUNK_SIZE"), 10, 64)
	if err != nil {
		logger.Fatal(err)
	}
	eventsChunkSize, err := strconv.ParseInt(os.Getenv("APP_EVENTS_CHUNK_SIZE"), 10, 64)
	if err != nil {
		logger.Fatal(err)
	}
	stakeChunkSize, err := strconv.ParseInt(os.Getenv("APP_STAKE_CHUNK_SIZE"), 10, 64)
	if err != nil {
		logger.Fatal(err)
	}
	wrkSaveTxsCount, err := strconv.ParseInt(os.Getenv("WRK_SAVE_TXS"), 10, 64)
	if err != nil {
		logger.Fatal(err)
	}
	wrkSaveTxsOutputCount, err := strconv.ParseInt(os.Getenv("WRK_SAVE_TXS_OUTPUT"), 10, 64)
	if err != nil {
		logger.Fatal(err)
	}
	wrkSaveInvalidTxsCount, err := strconv.ParseInt(os.Getenv("WRK_SAVE_TXS_INVALID"), 10, 64)
	if err != nil {
		logger.Fatal(err)
	}
	wrkSaveRewardsCount, err := strconv.ParseInt(os.Getenv("WRK_SAVE_REWARDS"), 10, 64)
	if err != nil {
		logger.Fatal(err)
	}
	wrkSaveSlashesCount, err := strconv.ParseInt(os.Getenv("WRK_SAVE_SLASHES"), 10, 64)
	if err != nil {
		logger.Fatal(err)
	}
	wrkSaveAddressesCount, err := strconv.ParseInt(os.Getenv("WRK_SAVE_ADDRESSES"), 10, 64)
	if err != nil {
		logger.Fatal(err)
	}
	wrkSaveValidatorTxsCount, err := strconv.ParseInt(os.Getenv("WRK_SAVE_TXS_VALIDATOR"), 10, 64)
	if err != nil {
		logger.Fatal(err)
	}
	addrChunkSize, err := strconv.ParseInt(os.Getenv("APP_ADDRESS_CHUNK_SIZE"), 10, 64)
	if err != nil {
		logger.Fatal(err)
	}
	wrkUpdateBalanceCount, err := strconv.ParseInt(os.Getenv("WRK_BALANCE"), 10, 64)
	if err != nil {
		logger.Fatal(err)
	}
	wrkGetBalancesFromNodeCount, err := strconv.ParseInt(os.Getenv("WRK_BALANCE_NODE"), 10, 64)
	if err != nil {
		logger.Fatal(err)
	}
	coinsUpdateTime, err := strconv.ParseInt(os.Getenv("APP_COINS_UPDATE_TIME_MINUTES"), 10, 64)
	if err != nil {
		logger.Fatal(err)
	}
	wrkUpdateTxsIndexNumBlocks, err := strconv.ParseInt(os.Getenv("WRK_TXS_INDEX_NUM"), 10, 64)
	if err != nil {
		logger.Fatal(err)
	}
	wrkUpdateTxsIndexTime, err := strconv.ParseInt(os.Getenv("WRK_TXS_INDEX_SLEEP_SEC"), 10, 64)
	if err != nil {
		logger.Fatal(err)
	}
	rewardAggregateEveryBlocksCount, err := strconv.ParseInt(os.Getenv("APP_REWARDS_AGGREGATE_BLOCKS_COUNT"), 10, 64)
	if err != nil {
		logger.Fatal(err)
	}
	extenderApiPort, err := strconv.ParseInt(os.Getenv("EXTENDER_API_PORT"), 10, 64)
	if err != nil {
		logger.Fatal(err)
	}

	envData := new(ExtenderEnvironment)
	envData.Debug = os.Getenv("EXTENDER_DEBUG") == "1"
	envData.BaseCoin = os.Getenv("MINTER_BASE_COIN")
	envData.DbHost = os.Getenv("DB_HOST")
	envData.DbPort = os.Getenv("DB_PORT")
	envData.DbName = os.Getenv("DB_NAME")
	envData.DbUser = os.Getenv("DB_USER")
	envData.DbPassword = os.Getenv("DB_PASSWORD")
	envData.NodeApi = os.Getenv("NODE_API")
	envData.WsLink = os.Getenv("CENTRIFUGO_LINK")
	envData.WsKey = os.Getenv("CENTRIFUGO_SECRET")
	envData.RewardAggregateTimeInterval = os.Getenv("APP_REWARDS_TIME_INTERVAL")
	envData.TxChunkSize = int(txChunkSize)
	envData.EventsChunkSize = int(eventsChunkSize)
	envData.WrkSaveTxsCount = int(wrkSaveTxsCount)
	envData.WrkSaveTxsOutputCount = int(wrkSaveTxsOutputCount)
	envData.WrkSaveInvTxsCount = int(wrkSaveInvalidTxsCount)
	envData.WrkSaveRewardsCount = int(wrkSaveRewardsCount)
	envData.WrkSaveSlashesCount = int(wrkSaveSlashesCount)
	envData.WrkSaveAddressesCount = int(wrkSaveAddressesCount)
	envData.WrkSaveValidatorTxsCount = int(wrkSaveValidatorTxsCount)
	envData.AddrChunkSize = int(addrChunkSize)
	envData.WrkUpdateBalanceCount = int(wrkUpdateBalanceCount)
	envData.WrkGetBalancesFromNodeCount = int(wrkGetBalancesFromNodeCount)
	envData.CoinsUpdateTime = int(coinsUpdateTime)
	envData.StakeChunkSize = int(stakeChunkSize)
	envData.WrkUpdateTxsIndexNumBlocks = int(wrkUpdateTxsIndexNumBlocks)
	envData.WrkUpdateTxsIndexTime = int(wrkUpdateTxsIndexTime)
	envData.RewardAggregateEveryBlocksCount = uint64(rewardAggregateEveryBlocksCount)
	envData.ApiPort = int(extenderApiPort)
	return envData
}
