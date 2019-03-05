package main

import (
	"flag"
	"github.com/MinterTeam/minter-explorer-extender/api"
	"github.com/MinterTeam/minter-explorer-extender/core"
	"github.com/MinterTeam/minter-explorer-extender/env"
	"os"
)

func main() {
	envData := initEnvironment()

	extenderApi := api.New(envData.ApiHost, envData.ApiPort)
	go extenderApi.Run()

	ext := core.NewExtender(envData)
	ext.Run()
}

func initEnvironment() *core.ExtenderEnvironment {
	debug := flag.Bool("debug", false, "Debug mode")
	dbName := flag.String("db_name", "", "DB name")
	dbUser := flag.String("db_user", "", "DB user")
	dbPassword := flag.String("db_password", "", "DB password")
	nodeApi := flag.String("node_api", "", "DB password")
	txChunkSize := flag.Int("tx_chunk_size", 100, "Transactions chunk size")
	configFile := flag.String("config", "", "Env file")
	apiHost := flag.String("extenderApi.host", "", "API host")
	apiPort := flag.Int("extenderApi.port", 8000, "API port")
	flag.Parse()

	envData := new(core.ExtenderEnvironment)

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
		config := env.NewViperConfig(*configFile)
		nodeApi := "http://"
		if config.GetBool("minterApi.isSecure") {
			nodeApi = "https://"
		}
		nodeApi += config.GetString("minterApi.link") + ":" + config.GetString("minterApi.port")
		envData.Debug = config.GetBool("debug")
		envData.DbName = config.GetString("database.name")
		envData.DbUser = config.GetString("database.user")
		envData.DbPassword = config.GetString("database.password")
		envData.NodeApi = nodeApi
		envData.TxChunkSize = *txChunkSize
		envData.ApiHost = config.GetString("extenderApi.host")
		envData.ApiPort = config.GetInt("extenderApi.port")
	} else {
		envData.Debug = *debug
		envData.DbName = *dbName
		envData.DbUser = *dbUser
		envData.DbPassword = *dbPassword
		envData.NodeApi = *nodeApi
		envData.TxChunkSize = *txChunkSize
		envData.ApiHost = *apiHost
		envData.ApiPort = *apiPort
	}

	return envData
}
