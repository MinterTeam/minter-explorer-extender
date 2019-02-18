package events

import (
	"github.com/MinterTeam/minter-explorer-extender/address"
	"github.com/MinterTeam/minter-explorer-extender/coin"
	"github.com/MinterTeam/minter-explorer-extender/helpers"
	"github.com/MinterTeam/minter-explorer-extender/models"
	"github.com/MinterTeam/minter-explorer-extender/validator"
	"github.com/daniildulin/minter-node-api/responses"
)

type Service struct {
	repository          *Repository
	validatorRepository *validator.Repository
	addressRepository   *address.Repository
	coinRepository      *coin.Repository
}

func NewService(repository *Repository, validatorRepository *validator.Repository, addressRepository *address.Repository, coinRepository *coin.Repository) *Service {
	return &Service{
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
		err     error
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
		err = s.repository.SaveRewards(rewards)
		if err != nil {
			return err
		}
	}

	if len(slashes) > 0 {
		err = s.repository.SaveSlashes(slashes)
		if err != nil {
			return err
		}
	}

	return nil
}
