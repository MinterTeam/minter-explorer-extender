package main

import (
	"flag"
	"github.com/MinterTeam/minter-explorer-extender/v2/core"
	"github.com/MinterTeam/minter-explorer-extender/v2/env"
	"github.com/MinterTeam/minter-explorer-extender/v2/metrics"
	"github.com/joho/godotenv"
	"log"
	"os"
)

var version = flag.Bool("version", false, "Prints current version")

func main() {
	flag.Parse()

	err := godotenv.Load()
	if err != nil {
		log.Println(".env file not found")
	}

	envData := env.New()
	metricsApi := metrics.New("", envData.ApiPort)
	go metricsApi.Run()
	ext := core.NewExtender(envData)

	if *version {
		ext.GetInfo()
		os.Exit(0)
	}

	ext.Run()
}
