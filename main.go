package main

import (
	"flag"
	"github.com/MinterTeam/minter-explorer-extender/core"
	"github.com/MinterTeam/minter-explorer-extender/env"
	"os"
)

func main() {
	envData := initEnvironment()
	ext := core.NewExtender(envData)
	ext.Run()
}

func initEnvironment() *core.ExtenderEnvironment {
	dbName := flag.String("db_name", "", "DB name")
	dbUser := flag.String("db_user", "", "DB user")
	dbPassword := flag.String("db_password", "", "DB password")
	nodeApi := flag.String("node_api", "", "DB password")
	txChunkSize := flag.Int("tx_chunk_size", 100, "Transactions chunk size")
	configFile := flag.String("config", "", "Env file")
	flag.Parse()

	envData := &core.ExtenderEnvironment{
		DbName:      *dbName,
		DbUser:      *dbUser,
		DbPassword:  *dbPassword,
		NodeApi:     *nodeApi,
		TxChunkSize: *txChunkSize,
	}

	if envData.DbUser == `` {
		dbUser := os.Getenv("EXPLORER_DB_USER")
		envData.DbUser = dbUser
	}
	if envData.DbName == `` {
		dbName := os.Getenv("EXPLORER_DB_NAME")
		envData.DbName = dbName
	}
	if envData.DbPassword == `` {
		dbPassword := os.Getenv("EXPLORER_DB_PASSWORD")
		envData.DbPassword = dbPassword
	}
	if envData.NodeApi == `` {
		nodeApi := os.Getenv("MINTER_NODE_API")
		envData.NodeApi = nodeApi
	}

	if *configFile != "" {
		config := env.NewViperConfig(*configFile)
		api := "http://"
		if config.GetBool("minterApi.isSecure") {
			api = "https://"
		}
		api += "http://" + config.GetString(`minterApi.link`) + `:` + config.GetString(`minterApi.port`)
		envData.DbName = config.GetString("database.name")
		envData.DbUser = config.GetString("database.user")
		envData.DbPassword = config.GetString("database.password")
		envData.NodeApi = api
		envData.TxChunkSize = *txChunkSize
	}
	return envData
}
