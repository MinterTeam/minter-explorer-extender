package main

import (
	"flag"
	"github.com/MinterTeam/minter-explorer-extender/core"
	"os"
)

func main() {
	env := initEnvironment()
	ext := core.NewExtender(env)
	ext.Run()
}

func initEnvironment() *core.ExtenderEnvironment {
	env := &core.ExtenderEnvironment{
		DbName:     flag.String("db_name", "", "DB name"),
		DbUser:     flag.String("db_user", "", "DB user"),
		DbPassword: flag.String("db_password", "", "DB password"),
		NodeApi:    flag.String("node_api", "", "DB password"),
	}
	flag.Parse()

	if *env.DbUser == `` {
		dbUser := os.Getenv("EXPLORER_DB_USER")
		env.DbUser = &dbUser
	}
	if *env.DbName == `` {
		dbName := os.Getenv("EXPLORER_DB_NAME")
		env.DbName = &dbName
	}
	if *env.DbPassword == `` {
		dbPassword := os.Getenv("EXPLORER_DB_PASSWORD")
		env.DbPassword = &dbPassword
	}
	if *env.NodeApi == `` {
		nodeApi := os.Getenv("MINTER_NODE_API")
		env.NodeApi = &nodeApi
	}

	return env
}
