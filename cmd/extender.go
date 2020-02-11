package main

import (
	"github.com/MinterTeam/minter-explorer-extender/api"
	"github.com/MinterTeam/minter-explorer-extender/core"
	"github.com/MinterTeam/minter-explorer-extender/env"
	"github.com/joho/godotenv"
	"log"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println(".env file not found")
	}
	envData := env.New()
	extenderApi := api.New(envData.ApiHost, envData.ApiPort)
	go extenderApi.Run()
	ext := core.NewExtender(envData)
	ext.Run()
}
