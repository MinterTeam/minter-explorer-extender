package env

import (
	"flag"
	"github.com/MinterTeam/minter-explorer-tools/models"
	"os"
)

func New() *models.ExtenderEnvironment {
	appName := flag.String("app_name", "Minter Extender", "App name")
	debug := flag.Bool("debug", false, "Debug mode")
	dbName := flag.String("db_name", "", "DB name")
	dbUser := flag.String("db_user", "", "DB user")
	dbPassword := flag.String("db_password", "", "DB password")
	dbMinIdleConns := flag.Int("db_min_idle_conns", 10, "DB min idle connections")
	dbPoolSize := flag.Int("db_pool_size", 20, "DB pool size")
	nodeApi := flag.String("node_api", "", "DB password")
	txChunkSize := flag.Int("tx_chunk_size", 100, "Transactions chunk size")
	eventsChunkSize := flag.Int("event_chunk_size", 100, "Events chunk size")
	configFile := flag.String("config", "", "Env file")
	apiHost := flag.String("api_host", "", "API host")
	apiPort := flag.Int("api_port", 8000, "API port")
	wsLink := flag.String("ws_link", "", "WebSocket server link")
	wsKey := flag.String("ws_key", "", "WebSocket API key")
	wrkSaveTxsCount := flag.Int("wrk_save_txs_count", 3, "Count of workers that save transactions")
	wrkSaveTxsOutputCount := flag.Int("wrk_save_txs_output_count", 3, "Count of workers that save transactions output")
	wrkSaveInvalidTxsCount := flag.Int("wrk_save_invtxs_count", 3, "Count of workers that save invalid transactions")
	wrkSaveRewardsCount := flag.Int("wrk_save_rewards_count", 3, "Count of workers that save rewards")
	wrkSaveSlashesCount := flag.Int("wrk_save_slashes_count", 3, "Count of workers that save slashes")
	wrkSaveAddressesCount := flag.Int("wrk_save_addresses_count", 3, "Count of workers that save addresses")
	wrkSaveValidatorTxsCount := flag.Int("wrk_save_val_tx_count", 3, "Count of workers that save transaction-validator link")
	addrChunkSize := flag.Int("addr_chunk_size", 10, "Count of workers that save transaction-validator link")
	wrkUpdateBalanceCount := flag.Int("wrk_upd_balances_count", 1, "Count of workers that update balance")
	wrkGetBalancesFromNodeCount := flag.Int("wrk_node_balance_count", 1, "Count of workers that get balance from node ")

	flag.Parse()

	envData := new(models.ExtenderEnvironment)

	if envData.DbUser == "" {
		dbUser := os.Getenv("EXPLORER_DB_USER")
		envData.DbUser = dbUser
	}
	if envData.DbName == "" {
		dbName := os.Getenv("EXPLORER_DB_NAME")
		envData.DbName = dbName
	}
	if envData.DbPassword == "" {
		dbPassword := os.Getenv("EXPLORER_DB_PASSWORD")
		envData.DbPassword = dbPassword
	}
	if envData.NodeApi == "" {
		nodeApi := os.Getenv("MINTER_NODE_API")
		envData.NodeApi = nodeApi
	}

	if *configFile != "" {
		config := NewViperConfig(*configFile)
		wsLink := `http://`
		if config.GetBool(`wsServer.isSecure`) {
			wsLink = `https://`
		}
		wsLink += config.GetString(`wsServer.link`)
		if config.GetString(`wsServer.port`) != `` {
			wsLink += `:` + config.GetString(`wsServer.port`)
		}
		nodeApi := "http://"
		if config.GetBool("minterApi.isSecure") {
			nodeApi = "https://"
		}
		nodeApi += config.GetString("minterApi.link") + ":" + config.GetString("minterApi.port")
		envData.Debug = config.GetBool("app.debug")
		envData.DbName = config.GetString("database.name")
		envData.DbUser = config.GetString("database.user")
		envData.DbPassword = config.GetString("database.password")
		envData.DbMinIdleConns = config.GetInt("database.minIdleConns")
		envData.DbPoolSize = config.GetInt("database.poolSize")
		envData.NodeApi = nodeApi
		envData.TxChunkSize = config.GetInt("app.txChunkSize")
		envData.AddrChunkSize = config.GetInt("app.addrChunkSize")
		envData.EventsChunkSize = config.GetInt("app.eventsChunkSize")
		envData.ApiHost = config.GetString("extenderApi.host")
		envData.ApiPort = config.GetInt("extenderApi.port")
		envData.WsLink = wsLink
		envData.WsKey = config.GetString(`wsServer.key`)
		envData.AppName = config.GetString("name")
		envData.WrkSaveTxsCount = config.GetInt("workers.saveTxs")
		envData.WrkSaveTxsOutputCount = config.GetInt("workers.saveTxsOutput")
		envData.WrkSaveInvTxsCount = config.GetInt("workers.saveInvalidTxs")
		envData.WrkSaveRewardsCount = config.GetInt("workers.saveRewards")
		envData.WrkSaveSlashesCount = config.GetInt("workers.saveSlashes")
		envData.WrkSaveAddressesCount = config.GetInt("workers.saveAddresses")
		envData.WrkSaveValidatorTxsCount = config.GetInt("workers.saveTxValidator")
		envData.WrkUpdateBalanceCount = config.GetInt("workers.updateBalance")
		envData.WrkGetBalancesFromNodeCount = config.GetInt("workers.balancesFromNode")
	} else {
		envData.AppName = *appName
		envData.Debug = *debug
		envData.DbName = *dbName
		envData.DbUser = *dbUser
		envData.DbPassword = *dbPassword
		envData.DbMinIdleConns = *dbMinIdleConns
		envData.DbPoolSize = *dbPoolSize
		envData.NodeApi = *nodeApi
		envData.TxChunkSize = *txChunkSize
		envData.EventsChunkSize = *eventsChunkSize
		envData.ApiHost = *apiHost
		envData.ApiPort = *apiPort
		envData.WsLink = *wsLink
		envData.WsKey = *wsKey
		envData.WrkSaveTxsCount = *wrkSaveTxsCount
		envData.WrkSaveTxsOutputCount = *wrkSaveTxsOutputCount
		envData.WrkSaveInvTxsCount = *wrkSaveInvalidTxsCount
		envData.WrkSaveRewardsCount = *wrkSaveRewardsCount
		envData.WrkSaveSlashesCount = *wrkSaveSlashesCount
		envData.WrkSaveAddressesCount = *wrkSaveAddressesCount
		envData.WrkSaveValidatorTxsCount = *wrkSaveValidatorTxsCount
		envData.AddrChunkSize = *addrChunkSize
		envData.WrkUpdateBalanceCount = *wrkUpdateBalanceCount
		envData.WrkGetBalancesFromNodeCount = *wrkGetBalancesFromNodeCount
	}

	return envData
}
