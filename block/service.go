package block

import (
	"github.com/MinterTeam/minter-explorer-extender/v2/broadcast"
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/MinterTeam/minter-explorer-extender/v2/validator"
	"github.com/MinterTeam/minter-explorer-tools/v4/helpers"
	"github.com/MinterTeam/node-grpc-gateway/api_pb"
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
	var proposerId uint
	var err error
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

	blockReward := "0"
	if response.BlockReward != nil {
		blockReward = response.BlockReward.Value
	}

	block := &models.Block{
		ID:                  response.Height,
		Size:                response.Size,
		BlockTime:           s.getBlockTime(blockTime),
		CreatedAt:           blockTime,
		BlockReward:         blockReward,
		ProposerValidatorID: uint64(proposerId),
		NumTxs:              uint32(response.TransactionCount),
		Hash:                response.Hash,
	}
	s.SetBlockCache(block)

	//need for correct broadcast model
	broadcastBlock := *block
	var blockValidators []models.BlockValidator
	for _, v := range response.Validators {
		blockValidators = append(blockValidators, models.BlockValidator{
			Validator: models.Validator{
				PublicKey: v.PublicKey,
			},
		})
	}
	broadcastBlock.BlockValidators = blockValidators
	s.broadcastService.BlockChannel() <- broadcastBlock

	return s.blockRepository.Save(block)
}

func (s *Service) getBlockTime(blockTime time.Time) uint64 {
	if s.blockCache == nil {
		return uint64(1 * time.Second) //ns, 1 second for the first block
	}
	result := blockTime.Sub(s.blockCache.CreatedAt)
	return uint64(result.Nanoseconds())
}
