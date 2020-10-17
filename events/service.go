package events

import (
	"fmt"
	"github.com/MinterTeam/minter-explorer-extender/v2/address"
	"github.com/MinterTeam/minter-explorer-extender/v2/balance"
	"github.com/MinterTeam/minter-explorer-extender/v2/coin"
	"github.com/MinterTeam/minter-explorer-extender/v2/env"
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/MinterTeam/minter-explorer-extender/v2/validator"
	"github.com/MinterTeam/minter-explorer-tools/v4/helpers"
	"github.com/MinterTeam/node-grpc-gateway/api_pb"
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
func (s *Service) HandleEventResponse(blockHeight uint64, responseEvents *api_pb.EventsResponse) error {
	var (
		rewards           []*models.Reward
		slashes           []*models.Slash
		coinsForUpdateMap = make(map[uint64]struct{})
	)

	for _, event := range responseEvents.Events {

		if fmt.Sprintf("%s", event.AsMap()["type"]) == "minter/UnbondEvent" {
			continue
		}

		if fmt.Sprintf("%s", event.AsMap()["type"]) == "minter/StakeKickEvent" {
			mapValues := event.AsMap()["value"].(map[string]interface{})

			addressId, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(mapValues["address"].(string)))
			if err != nil {
				s.logger.Error(err)
				continue
			}

			vId, err := s.validatorRepository.FindIdByPk(helpers.RemovePrefix(mapValues["validator_pub_key"].(string)))
			if err != nil {
				s.logger.Error(err)
				continue
			}
			cid, err := strconv.ParseUint(mapValues["coin"].(string), 10, 64)
			if err != nil {
				s.logger.Error(err)
				continue
			}

			coinsForUpdateMap[cid] = struct{}{}
			stk := &models.Stake{
				OwnerAddressID: addressId,
				CoinID:         uint(cid),
				ValidatorID:    vId,
				Value:          mapValues["amount"].(string),
				IsKicked:       true,
				BipValue:       "0",
			}

			err = s.validatorRepository.UpdateStake(stk)
			if err != nil {
				s.logger.Error(err)
			}

			continue
		}

		values := event.AsMap()["value"].(map[string]interface{})
		addressId, err := s.addressRepository.FindId(helpers.RemovePrefix(values["address"].(string)))
		if err != nil {
			s.logger.WithFields(logrus.Fields{
				"address": values["address"].(string),
			}).Error(err)
			return err
		}

		validatorId, err := s.validatorRepository.FindIdByPk(helpers.RemovePrefix(values["validator_pub_key"].(string)))
		if err != nil {
			s.logger.WithFields(logrus.Fields{
				"public_key": values["validator_pub_key"],
			}).Error(err)
			return err
		}

		switch fmt.Sprintf("%s", event.AsMap()["type"]) {
		case models.RewardEvent:
			rewards = append(rewards, &models.Reward{
				BlockID:     blockHeight,
				Role:        values["role"].(string),
				Amount:      values["amount"].(string),
				AddressID:   addressId,
				ValidatorID: uint64(validatorId),
			})

		case models.SlashEvent:

			coinId, err := strconv.ParseUint(values["coin"].(string), 10, 64)
			if err != nil {
				continue
			}

			coinsForUpdateMap[coinId] = struct{}{}
			slashes = append(slashes, &models.Slash{
				BlockID:     blockHeight,
				CoinID:      uint(coinId),
				Amount:      values["amount"].(string),
				AddressID:   addressId,
				ValidatorID: uint64(validatorId),
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
