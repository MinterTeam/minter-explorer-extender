package main

import (
	"github.com/MinterTeam/minter-explorer-extender/v2/api"
	"github.com/MinterTeam/minter-explorer-extender/v2/core"
	"github.com/MinterTeam/minter-explorer-extender/v2/env"
	"github.com/joho/godotenv"
	"log"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println(".env file not found")
	}
	envData := env.New()
	extenderApi := api.New("", envData.ApiPort)
	go extenderApi.Run()
	ext := core.NewExtender(envData)
	ext.Run()
}
