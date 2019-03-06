package events

import (
	"github.com/MinterTeam/minter-explorer-extender/address"
	"github.com/MinterTeam/minter-explorer-extender/coin"
	"github.com/MinterTeam/minter-explorer-extender/helpers"
	"github.com/MinterTeam/minter-explorer-extender/models"
	"github.com/MinterTeam/minter-explorer-extender/validator"
	"github.com/daniildulin/minter-node-api/responses"
	"math"
)

type Service struct {
	env                 *models.ExtenderEnvironment
	repository          *Repository
	validatorRepository *validator.Repository
	addressRepository   *address.Repository
	coinRepository      *coin.Repository
}

func NewService(env *models.ExtenderEnvironment, repository *Repository, validatorRepository *validator.Repository, addressRepository *address.Repository, coinRepository *coin.Repository) *Service {
	return &Service{
		env:                 env,
		repository:          repository,
		validatorRepository: validatorRepository,
		addressRepository:   addressRepository,
		coinRepository:      coinRepository,
	}
}

//Handle response and save block to DB
func (s *Service) HandleEventResponse(blockHeight uint64, response *responses.EventsResponse) error {
	var (
		rewards []*models.Reward
		slashes []*models.Slash
	)

	for _, event := range response.Result.Events {
		addressId, err := s.addressRepository.FindId(helpers.RemovePrefix(event.Value.Address))
		if err != nil {
			return err
		}

		validatorId, err := s.validatorRepository.FindIdByPk(helpers.RemovePrefix(event.Value.ValidatorPubKey))
		if err != nil {
			return err
		}

		switch event.Type {
		case models.RewardEvent:

			rewards = append(rewards, &models.Reward{
				BlockID:     blockHeight,
				Role:        event.Value.Role,
				Amount:      event.Value.Amount,
				AddressID:   addressId,
				ValidatorID: validatorId,
			})

		case models.SlashEvent:
			coinId, err := s.coinRepository.FindIdBySymbol(event.Value.Coin)
			if err != nil {
				return err
			}

			slashes = append(slashes, &models.Slash{
				BlockID:     blockHeight,
				CoinID:      coinId,
				Amount:      event.Value.Amount,
				AddressID:   addressId,
				ValidatorID: validatorId,
			})
		}
	}

	if len(rewards) > 0 {
		s.saveRewards(rewards)
	}

	if len(slashes) > 0 {
		s.saveSlashes(slashes)
	}

	return nil
}

func (s Service) saveRewards(rewards []*models.Reward) {
	chunksCount := int(math.Ceil(float64(len(rewards)) / float64(s.env.EventsChunkSize)))
	chunks := make([][]*models.Reward, chunksCount)
	for i := 0; i < chunksCount; i++ {
		start := s.env.EventsChunkSize * i
		end := start + s.env.EventsChunkSize
		if end > len(rewards) {
			end = len(rewards)
		}
		chunks[i] = rewards[start:end]
	}

	for _, chunk := range chunks {
		go func() {
			err := s.repository.SaveRewards(chunk)
			helpers.HandleError(err)
		}()
	}
}

func (s Service) saveSlashes(slashes []*models.Slash) {
	chunksCount := int(math.Ceil(float64(len(slashes)) / float64(s.env.EventsChunkSize)))
	chunks := make([][]*models.Slash, chunksCount)
	for i := 0; i < chunksCount; i++ {
		start := s.env.EventsChunkSize * i
		end := start + s.env.EventsChunkSize
		if end > len(slashes) {
			end = len(slashes)
		}
		chunks[i] = slashes[start:end]
	}

	for _, chunk := range chunks {
		go func() {
			err := s.repository.SaveSlashes(chunk)
			helpers.HandleError(err)
		}()
	}
}
