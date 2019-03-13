package block

import (
	"github.com/MinterTeam/minter-explorer-extender/validator"
	"github.com/MinterTeam/minter-explorer-tools/helpers"
	"github.com/MinterTeam/minter-explorer-tools/models"
	"github.com/daniildulin/minter-node-api/responses"
	"strconv"
	"time"
)

type Service struct {
	blockRepository     *Repository
	validatorRepository *validator.Repository
	blockCache          *models.Block //Contain previous block model
}

func NewBlockService(blockRepository *Repository, validatorRepository *validator.Repository) *Service {
	return &Service{
		blockRepository:     blockRepository,
		validatorRepository: validatorRepository,
	}
}

func (s *Service) SetBlockCache(b *models.Block) {
	s.blockCache = b
}

func (s *Service) GetBlockCache() (b *models.Block) {
	return s.blockCache
}

//Handle response and save block to DB
func (s *Service) HandleBlockResponse(response *responses.BlockResponse) error {
	height, err := strconv.ParseUint(response.Result.Height, 10, 64)
	helpers.HandleError(err)
	totalTx, err := strconv.ParseUint(response.Result.TotalTx, 10, 64)
	helpers.HandleError(err)
	numTx, err := strconv.ParseUint(response.Result.TxCount, 10, 32)
	helpers.HandleError(err)
	size, err := strconv.ParseUint(response.Result.Size, 10, 64)
	helpers.HandleError(err)
	proposerId, err := s.validatorRepository.FindIdByPk(helpers.RemovePrefix(response.Result.Proposer))
	helpers.HandleError(err)
	block := &models.Block{
		ID:                  height,
		TotalTxs:            totalTx,
		NumTxs:              uint32(numTx),
		Size:                size,
		BlockTime:           s.getBlockTime(response.Result.Time),
		CreatedAt:           response.Result.Time,
		BlockReward:         response.Result.BlockReward,
		ProposerValidatorID: proposerId,
		Hash:                response.Result.Hash,
	}
	s.SetBlockCache(block)
	return s.blockRepository.Save(block)
}

func (s *Service) getBlockTime(blockTime time.Time) uint64 {
	if s.blockCache == nil {
		return uint64(1 * time.Second) //ns, 1 second for the first block
	}
	result := blockTime.Sub(s.blockCache.CreatedAt)
	return uint64(result.Nanoseconds())
}
