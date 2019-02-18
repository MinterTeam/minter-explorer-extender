package events

import (
	"errors"
	"github.com/MinterTeam/minter-explorer-extender/address"
	"github.com/MinterTeam/minter-explorer-extender/coin"
	"github.com/MinterTeam/minter-explorer-extender/models"
	"github.com/MinterTeam/minter-explorer-extender/validator"
	"github.com/daniildulin/minter-node-api/responses"
	"math/big"
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
		rewardsMap = make(map[string]*models.Reward)
		slashes    []*models.Slash
		err        error
	)

	for _, event := range response.Result.Events {
		a := []rune(event.Value.Address)
		addressId, err := s.addressRepository.FindId(string(a[2:]))
		if err != nil {
			return err
		}
		v := []rune(event.Value.ValidatorPubKey)
		validatorId, err := s.validatorRepository.FindIdByPk(string(v[2:]))
		if err != nil {
			return err
		}

		if event.Type == "minter/RewardEvent" {
			if rewardsMap[event.Value.Address] == nil {
				rewardsMap[event.Value.Address] = &models.Reward{
					BlockID:     blockHeight,
					Role:        event.Value.Role,
					Amount:      event.Value.Amount,
					AddressID:   addressId,
					ValidatorID: validatorId,
				}
			} else {
				oldAmount := new(big.Int)
				oldAmount, ok := oldAmount.SetString(rewardsMap[event.Value.Address].Amount, 10)
				if !ok {
					return errors.New("error parse reward amount")
				}
				amount := new(big.Int)
				amount, ok = amount.SetString(event.Value.Amount, 10)
				if !ok {
					return errors.New("error parse reward amount")
				}

				amount.Add(amount, oldAmount)
				rewardsMap[event.Value.Address].Amount = amount.String()
			}
		} else if event.Type == "minter/SlashEvent" {
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

	if len(rewardsMap) > 0 {
		rewards := make([]*models.Reward, len(rewardsMap))
		i := 0
		for _, reward := range rewardsMap {
			rewards[i] = reward
			i++
		}
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
