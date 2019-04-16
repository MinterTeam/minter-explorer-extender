package events

import (
	"github.com/MinterTeam/minter-explorer-extender/address"
	"github.com/MinterTeam/minter-explorer-extender/coin"
	"github.com/MinterTeam/minter-explorer-extender/validator"
	"github.com/MinterTeam/minter-explorer-tools/helpers"
	"github.com/MinterTeam/minter-explorer-tools/models"
	"github.com/MinterTeam/minter-node-go-api/responses"
	"math"
)

type Service struct {
	env                 *models.ExtenderEnvironment
	repository          *Repository
	validatorRepository *validator.Repository
	addressRepository   *address.Repository
	coinRepository      *coin.Repository
	coinService         *coin.Service
	jobSaveRewards      chan []*models.Reward
	jobSaveSlashes      chan []*models.Slash
}

func NewService(env *models.ExtenderEnvironment, repository *Repository, validatorRepository *validator.Repository,
	addressRepository *address.Repository, coinRepository *coin.Repository, coinService *coin.Service) *Service {
	return &Service{
		env:                 env,
		repository:          repository,
		validatorRepository: validatorRepository,
		addressRepository:   addressRepository,
		coinRepository:      coinRepository,
		coinService:         coinService,
		jobSaveRewards:      make(chan []*models.Reward, env.WrkSaveRewardsCount),
		jobSaveSlashes:      make(chan []*models.Slash, env.WrkSaveSlashesCount),
	}
}

//Handle response and save block to DB
func (s *Service) HandleEventResponse(blockHeight uint64, response *responses.EventsResponse) error {
	var (
		rewards           []*models.Reward
		slashes           []*models.Slash
		coinsForUpdateMap = make(map[string]struct{})
	)

	for _, event := range response.Result.Events {
		if event.Type == "minter/CoinLiquidationEvent" {
			err := s.coinRepository.DeleteBySymbol(event.Value.Coin)
			if err != nil {
				return err
			}
			continue
		}

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
			coinsForUpdateMap[event.Value.Coin] = struct{}{}
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

	if len(coinsForUpdateMap) > 0 {
		s.coinService.GetUpdateCoinsFromCoinsMapJobChannel() <- coinsForUpdateMap
	}

	if len(rewards) > 0 {
		s.saveRewards(rewards)
	}

	if len(slashes) > 0 {
		s.saveSlashes(slashes)
	}

	return nil
}

func (s *Service) GetSaveRewardsJobChannel() chan []*models.Reward {
	return s.jobSaveRewards
}

func (s *Service) GetSaveSlashesJobChannel() chan []*models.Slash {
	return s.jobSaveSlashes
}

func (s *Service) SaveRewardsWorker(jobs <-chan []*models.Reward) {
	for rewards := range jobs {
		err := s.repository.SaveRewards(rewards)
		helpers.HandleError(err)
	}
}

func (s *Service) SaveSlashesWorker(jobs <-chan []*models.Slash) {
	for slashes := range jobs {
		err := s.repository.SaveSlashes(slashes)
		helpers.HandleError(err)
	}
}

func (s *Service) saveRewards(rewards []*models.Reward) {
	chunksCount := int(math.Ceil(float64(len(rewards)) / float64(s.env.EventsChunkSize)))
	for i := 0; i < chunksCount; i++ {
		start := s.env.EventsChunkSize * i
		end := start + s.env.EventsChunkSize
		if end > len(rewards) {
			end = len(rewards)
		}
		s.GetSaveRewardsJobChannel() <- rewards[start:end]
	}
}

func (s *Service) saveSlashes(slashes []*models.Slash) {
	chunksCount := int(math.Ceil(float64(len(slashes)) / float64(s.env.EventsChunkSize)))
	for i := 0; i < chunksCount; i++ {
		start := s.env.EventsChunkSize * i
		end := start + s.env.EventsChunkSize
		if end > len(slashes) {
			end = len(slashes)
		}
		s.GetSaveSlashesJobChannel() <- slashes[start:end]
	}
}
