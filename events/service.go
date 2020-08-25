package events

import (
	"github.com/MinterTeam/minter-explorer-extender/v2/address"
	"github.com/MinterTeam/minter-explorer-extender/v2/balance"
	"github.com/MinterTeam/minter-explorer-extender/v2/coin"
	"github.com/MinterTeam/minter-explorer-extender/v2/env"
	"github.com/MinterTeam/minter-explorer-extender/v2/validator"
	"github.com/MinterTeam/minter-explorer-tools/v4/helpers"
	"github.com/MinterTeam/minter-explorer-tools/v4/models"
	"github.com/MinterTeam/minter-go-sdk/api"
	"github.com/sirupsen/logrus"
	"math"
	"os"
	"strconv"
)

type Service struct {
	env                 *env.ExtenderEnvironment
	repository          *Repository
	validatorRepository *validator.Repository
	addressRepository   *address.Repository
	coinRepository      *coin.Repository
	coinService         *coin.Service
	balanceRepository   *balance.Repository
	jobSaveRewards      chan []*models.Reward
	jobSaveSlashes      chan []*models.Slash
	logger              *logrus.Entry
}

func NewService(env *env.ExtenderEnvironment, repository *Repository, validatorRepository *validator.Repository,
	addressRepository *address.Repository, coinRepository *coin.Repository, coinService *coin.Service,
	balanceRepository *balance.Repository, logger *logrus.Entry) *Service {
	return &Service{
		env:                 env,
		repository:          repository,
		validatorRepository: validatorRepository,
		addressRepository:   addressRepository,
		coinRepository:      coinRepository,
		coinService:         coinService,
		balanceRepository:   balanceRepository,
		jobSaveRewards:      make(chan []*models.Reward, env.WrkSaveRewardsCount),
		jobSaveSlashes:      make(chan []*models.Slash, env.WrkSaveSlashesCount),
		logger:              logger,
	}
}

//Handle response and save block to DB
func (s *Service) HandleEventResponse(blockHeight uint64, response *api.EventsResult) error {
	var (
		rewards           []*models.Reward
		slashes           []*models.Slash
		coinsForUpdateMap = make(map[string]struct{})
	)

	for _, event := range response.Events {
		if event.Type == "minter/CoinLiquidationEvent" {
			coinId, err := s.coinRepository.FindIdBySymbol(event.Value["coin"])
			err = s.balanceRepository.DeleteByCoinId(coinId)
			if err != nil {
				s.logger.WithFields(logrus.Fields{
					"coin": event.Value["coin"],
				}).Error(err)
				return err
			}

			err = s.coinRepository.DeleteBySymbol(event.Value["coin"])
			if err != nil {
				s.logger.WithFields(logrus.Fields{
					"coin": event.Value["coin"],
				}).Error(err)
				return err
			}
			continue
		}
		if event.Type == "minter/UnbondEvent" {
			continue
		}

		addressId, err := s.addressRepository.FindId(helpers.RemovePrefix(event.Value["address"]))
		if err != nil {
			s.logger.WithFields(logrus.Fields{
				"address": event.Value["address"],
			}).Error(err)
			return err
		}

		validatorId, err := s.validatorRepository.FindIdByPk(helpers.RemovePrefix(event.Value["validator_pub_key"]))
		if err != nil {
			s.logger.WithFields(logrus.Fields{
				"public_key": event.Value["validator_pub_key"],
			}).Error(err)
			return err
		}

		switch event.Type {
		case models.RewardEvent:
			rewards = append(rewards, &models.Reward{
				BlockID:     blockHeight,
				Role:        event.Value["role"],
				Amount:      event.Value["amount"],
				AddressID:   addressId,
				ValidatorID: validatorId,
			})

		case models.SlashEvent:
			coinsForUpdateMap[event.Value["coin"]] = struct{}{}
			coinId, err := s.coinRepository.FindIdBySymbol(event.Value["coin"])
			if err != nil {
				s.logger.Error(err)
				return err
			}

			slashes = append(slashes, &models.Slash{
				BlockID:     blockHeight,
				CoinID:      coinId,
				Amount:      event.Value["amount"],
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

func (s *Service) AggregateRewards(aggregateInterval string, beforeBlockId uint64) {
	err := s.repository.AggregateRewards(aggregateInterval, beforeBlockId)
	if err != nil {
		s.logger.Error(err)
	}
	helpers.HandleError(err)

	blocks := os.Getenv("APP_REWARDS_BLOCKS")
	bc, err := strconv.ParseUint(blocks, 10, 32)
	if err != nil {
		s.logger.Error(err)
		return
	}

	err = s.repository.DropOldRewardsData(uint32(bc))
	if err != nil {
		s.logger.Error(err)
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
