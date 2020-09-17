package block

import (
	"github.com/MinterTeam/minter-explorer-extender/v2/broadcast"
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/MinterTeam/minter-explorer-extender/v2/validator"
	"github.com/MinterTeam/minter-explorer-tools/v4/helpers"
	"github.com/MinterTeam/node-grpc-gateway/api_pb"
	"strconv"
	"time"
)

type Service struct {
	blockRepository     *Repository
	validatorRepository *validator.Repository
	broadcastService    *broadcast.Service
	blockCache          *models.Block //Contain previous block model
}

func NewBlockService(blockRepository *Repository, validatorRepository *validator.Repository, broadcastService *broadcast.Service) *Service {
	return &Service{
		blockRepository:     blockRepository,
		validatorRepository: validatorRepository,
		broadcastService:    broadcastService,
	}
}

func (s *Service) SetBlockCache(b *models.Block) {
	s.blockCache = b
}

func (s *Service) GetBlockCache() (b *models.Block) {
	return s.blockCache
}

//Handle response and save block to DB
func (s *Service) HandleBlockResponse(response *api_pb.BlockResponse) error {
	height, err := strconv.ParseUint(response.Height, 10, 64)
	if err != nil {
		return err
	}

	size, err := strconv.ParseUint(response.Size, 10, 64)
	if err != nil {
		return err
	}
	var proposerId uint
	if response.Proposer != "" {
		proposerId, err = s.validatorRepository.FindIdByPk(helpers.RemovePrefix(response.Proposer))
		helpers.HandleError(err)
	} else {
		proposerId = 1
	}

	layout := "2006-01-02T15:04:05Z"
	blockTime, err := time.Parse(layout, response.Time)
	if err != nil {
		return err
	}

	numTxs, err := strconv.ParseUint(response.TransactionCount, 10, 64)
	if err != nil {
		return err
	}

	block := &models.Block{
		ID:                  height,
		Size:                size,
		BlockTime:           s.getBlockTime(blockTime),
		CreatedAt:           blockTime,
		BlockReward:         response.BlockReward,
		ProposerValidatorID: uint64(proposerId),
		NumTxs:              uint32(numTxs),
		Hash:                response.Hash,
	}
	s.SetBlockCache(block)

	//todo
	go s.broadcastService.PublishBlock(block)
	go s.broadcastService.PublishStatus()

	return s.blockRepository.Save(block)
}

func (s *Service) getBlockTime(blockTime time.Time) uint64 {
	if s.blockCache == nil {
		return uint64(1 * time.Second) //ns, 1 second for the first block
	}
	result := blockTime.Sub(s.blockCache.CreatedAt)
	return uint64(result.Nanoseconds())
}
