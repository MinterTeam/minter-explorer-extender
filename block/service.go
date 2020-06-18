package block

import (
	"github.com/MinterTeam/minter-explorer-extender/v2/broadcast"
	"github.com/MinterTeam/minter-explorer-extender/v2/validator"
	"github.com/MinterTeam/minter-explorer-tools/v4/helpers"
	"github.com/MinterTeam/minter-explorer-tools/v4/models"
	"github.com/MinterTeam/minter-go-sdk/api"
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
func (s *Service) HandleBlockResponse(response *api.BlockResult) error {
	height, err := strconv.ParseUint(response.Height, 10, 64)
	helpers.HandleError(err)
	numTx, err := strconv.ParseUint(response.NumTxs, 10, 32)
	helpers.HandleError(err)
	size, err := strconv.ParseUint(response.Size, 10, 64)
	helpers.HandleError(err)

	var proposerId uint64
	if response.Proposer != "" {
		proposerId, err = s.validatorRepository.FindIdByPk(helpers.RemovePrefix(response.Proposer))
		helpers.HandleError(err)
	} else {
		proposerId = 1
	}

	block := &models.Block{
		ID:                  height,
		NumTxs:              uint32(numTx),
		Size:                size,
		BlockTime:           s.getBlockTime(response.Time),
		CreatedAt:           response.Time,
		BlockReward:         response.BlockReward,
		ProposerValidatorID: proposerId,
		Hash:                response.Hash,
	}
	s.SetBlockCache(block)

	go s.broadcastService.PublishBlock(block)

	go s.broadcastService.PublishTotalSlashes()

	return s.blockRepository.Save(block)
}

func (s *Service) getBlockTime(blockTime time.Time) uint64 {
	if s.blockCache == nil {
		return uint64(1 * time.Second) //ns, 1 second for the first block
	}
	result := blockTime.Sub(s.blockCache.CreatedAt)
	return uint64(result.Nanoseconds())
}
