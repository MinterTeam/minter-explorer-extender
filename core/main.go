package core

import (
	"fmt"
	"github.com/MinterTeam/minter-explorer-extender/block"
	"github.com/MinterTeam/minter-explorer-extender/helpers"
	"github.com/MinterTeam/minter-explorer-extender/models"
	"github.com/MinterTeam/minter-explorer-extender/validator"
	"github.com/daniildulin/minter-node-api"
	"github.com/daniildulin/minter-node-api/responses"
	"github.com/go-pg/pg"
	"log"
	"strconv"
	"time"
)

type ExtenderEnvironment struct {
	DbName     string
	DbUser     string
	DbPassword string
	NodeApi    string
}

type Extender struct {
	nodeApi             *minter_node_api.MinterNodeApi
	blockService        *block.Service
	blockRepository     *block.Repository
	validatorService    *validator.Service
	validatorRepository *validator.Repository
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

	blockRepository := block.NewRepository(db)
	validatorRepository := validator.NewRepository(db)

	return &Extender{
		nodeApi:             minter_node_api.New(env.NodeApi),
		blockRepository:     blockRepository,
		blockService:        block.NewBlockService(blockRepository, validatorRepository),
		validatorService:    validator.NewService(validatorRepository),
		validatorRepository: validatorRepository,
	}
}

func (ext *Extender) Run() {
	var i, startHeight uint64

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
		start := time.Now()
		resp, err := ext.nodeApi.GetBlock(i)
		helpers.HandleError(err)
		elapsed := time.Since(start)
		log.Println("Pulling block data")
		log.Printf("Processing time %s", elapsed)

		start = time.Now()
		ext.handleBlockResponse(resp)
		elapsed = time.Since(start)
		log.Println("Handle block")
		log.Printf("Processing time %s", elapsed)
	}

}

func (ext *Extender) handleBlockResponse(response *responses.BlockResponse) {
	//Save block
	err := ext.blockService.HandleBlockResponse(response)
	helpers.HandleError(err)

	//Save validators if not exist
	err = ext.validatorService.HandleBlockResponse(response)
	helpers.HandleError(err)

	err = ext.linkBlockValidator(response)
	helpers.HandleError(err)
}

func (ext *Extender) linkBlockValidator(response *responses.BlockResponse) error {
	var links []*models.BlockValidator

	height, err := strconv.ParseUint(response.Result.Height, 10, 64)
	if err != nil {
		return err
	}

	for _, v := range response.Result.Validators {
		pk := []rune(v.PubKey)
		vId, err := ext.validatorRepository.FindIdOrCreateByPk(string(pk[2:]))
		if err != nil {
			log.Println(err)
			continue
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
