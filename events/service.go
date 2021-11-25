package events

import (
	"encoding/json"
	"fmt"
	"github.com/MinterTeam/minter-explorer-extender/v2/address"
	"github.com/MinterTeam/minter-explorer-extender/v2/balance"
	"github.com/MinterTeam/minter-explorer-extender/v2/block"
	"github.com/MinterTeam/minter-explorer-extender/v2/broadcast"
	"github.com/MinterTeam/minter-explorer-extender/v2/coin"
	"github.com/MinterTeam/minter-explorer-extender/v2/env"
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/MinterTeam/minter-explorer-extender/v2/orderbook"
	"github.com/MinterTeam/minter-explorer-extender/v2/validator"
	"github.com/MinterTeam/minter-explorer-tools/v4/helpers"
	"github.com/MinterTeam/minter-go-sdk/v2/api"
	"github.com/MinterTeam/minter-go-sdk/v2/api/grpc_client"
	"github.com/MinterTeam/node-grpc-gateway/api_pb"
	"github.com/go-pg/pg/v10"
	"github.com/sirupsen/logrus"
	"math"
	"math/big"
	"strconv"
	"time"
)

type Service struct {
	env                 *env.ExtenderEnvironment
	repository          *Repository
	validatorRepository *validator.Repository
	addressRepository   *address.Repository
	coinRepository      *coin.Repository
	coinService         *coin.Service
	balanceRepository   *balance.Repository
	blockRepository     *block.Repository
	broadcastService    *broadcast.Service
	orderRepository     *orderbook.Repository
	jobSaveRewards      chan []*models.Reward
	jobSaveSlashes      chan []*models.Slash
	startBlock          uint64
	logger              *logrus.Entry
}

func NewService(env *env.ExtenderEnvironment, repository *Repository, validatorRepository *validator.Repository,
	addressRepository *address.Repository, coinRepository *coin.Repository, coinService *coin.Service,
	blockRepository *block.Repository, orderRepository *orderbook.Repository, balanceRepository *balance.Repository, broadcastService *broadcast.Service,
	logger *logrus.Entry, startBlock uint64) *Service {
	return &Service{
		env:                 env,
		repository:          repository,
		validatorRepository: validatorRepository,
		addressRepository:   addressRepository,
		coinRepository:      coinRepository,
		coinService:         coinService,
		balanceRepository:   balanceRepository,
		blockRepository:     blockRepository,
		orderRepository:     orderRepository,
		broadcastService:    broadcastService,
		startBlock:          startBlock,
		jobSaveRewards:      make(chan []*models.Reward, env.WrkSaveRewardsCount),
		jobSaveSlashes:      make(chan []*models.Slash, env.WrkSaveSlashesCount),
		logger:              logger,
	}
}

// HandleEventResponse Handle response and save block to DB
func (s *Service) HandleEventResponse(blockHeight uint64, responseEvents *api_pb.EventsResponse) error {
	var (
		eventList         []models.Event
		rewards           []*models.Reward
		slashes           []*models.Slash
		coinsForUpdateMap = make(map[uint64]struct{})
	)

	for _, event := range responseEvents.Events {
		eventStruct, err := grpc_client.ConvertStructToEvent(event)
		if err != nil {
			return err
		}

		jsonEvent, err := json.Marshal(eventStruct)
		if err != nil {
			return err
		}

		if fmt.Sprintf("%s", eventStruct.Type()) != "minter/RewardEvent" {
			eventList = append(eventList, models.Event{
				BlockId: blockHeight,
				Type:    fmt.Sprintf("%s", eventStruct.Type()),
				Data:    jsonEvent,
			})
		}

		switch e := eventStruct.(type) {
		case *api.RewardEvent:
			reward, err := s.handleRewardEvent(blockHeight, e)
			if err != nil {
				return err
			}
			rewards = append(rewards, reward)

		case *api.SlashEvent:
			coinId, err := strconv.ParseUint(e.Coin, 10, 64)
			if err != nil {
				s.logger.WithFields(logrus.Fields{
					"coin": e.Coin,
				}).Error(err)
				continue
			}
			addressId, err := s.addressRepository.FindId(helpers.RemovePrefix(e.Address))
			if err != nil {
				s.logger.WithFields(logrus.Fields{
					"address": e.Address,
				}).Error(err)
				continue
			}

			validatorId, err := s.validatorRepository.FindIdByPk(helpers.RemovePrefix(e.GetPublicKey()))
			if err != nil {
				s.logger.WithFields(logrus.Fields{
					"public_key": e.GetPublicKey(),
				}).Error(err)
				continue
			}
			coinsForUpdateMap[coinId] = struct{}{}
			slashes = append(slashes, &models.Slash{
				BlockID:     blockHeight,
				CoinID:      uint(coinId),
				Amount:      e.Amount,
				AddressID:   addressId,
				ValidatorID: uint64(validatorId),
			})
		case *api.StakeKickEvent:
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
		case *api.UnbondEvent:
			continue
		case *api.UpdateCommissionsEvent:
			s.broadcastService.CommissionsChannel() <- eventStruct
		case *api.JailEvent:
			validatorId, err := s.validatorRepository.FindIdByPk(helpers.RemovePrefix(e.GetPublicKey()))
			if err != nil {
				s.logger.Error(err)
				continue
			}

			blockId, err := strconv.ParseUint(e.JailedUntil, 10, 64)
			if err != nil {
				s.logger.Error(err)
				continue
			}

			ban := &models.ValidatorBan{
				ValidatorId: validatorId,
				BlockId:     blockHeight,
				ToBlockId:   blockId,
			}

			err = s.validatorRepository.SaveBan(ban)
			if err != nil {
				s.logger.Error(err)
			}
		case *api.OrderExpiredEvent:
			orderId, err := strconv.ParseUint(e.ID, 10, 64)
			if err != nil {
				s.logger.Error(err)
				continue
			}

			err = s.orderRepository.CancelByIdList([]uint64{orderId}, models.OrderTypeExpired)
			if err != nil {
				s.logger.Error(err)
			}
		}
	}

	if len(eventList) > 0 {
		err := s.repository.Add(eventList)
		if err != nil {
			s.logger.Error(err)
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
		//Temporary disabled
		continue

		b, err := s.blockRepository.GetById(rewards[0].BlockID)
		if err != nil {
			s.logger.Fatal(err)
		}

		timeId, err := time.Parse("2006-01-02", b.CreatedAt.Format("2006-01-02"))
		if err != nil {
			s.logger.Fatal(err)
		}

		exist, err := s.repository.GetRewardsByDay(timeId)
		if err != nil && err != pg.ErrNoRows {
			s.logger.Fatal(err)
		}

		rewardsMap := map[string][]*models.Reward{}
		for _, reward := range rewards {
			key := fmt.Sprintf("%d-%d-%s", reward.AddressID, reward.ValidatorID, reward.Role)
			rewardsMap[key] = append(rewardsMap[key], reward)
		}

		if (err != nil && err == pg.ErrNoRows) || len(exist) == 0 {
			startBlock := rewards[0].BlockID - 120
			if startBlock < s.startBlock {
				startBlock = s.startBlock
			}

			var aggregated []*models.AggregatedReward
			for _, userRewards := range rewardsMap {
				total := big.NewInt(0)
				for _, r := range userRewards {
					amount, _ := big.NewInt(0).SetString(r.Amount, 10)
					total.Add(total, amount)
				}

				aggregated = append(aggregated, &models.AggregatedReward{
					FromBlockID: startBlock,
					ToBlockID:   userRewards[0].BlockID,
					AddressID:   uint64(userRewards[0].AddressID),
					ValidatorID: userRewards[0].ValidatorID,
					Role:        userRewards[0].Role,
					Amount:      total.String(),
					TimeID:      timeId,
				})
			}
			err = s.repository.SaveAggregatedRewards(aggregated)
			helpers.HandleError(err)
			continue
		}

		existRewardsMap := map[string]*models.AggregatedReward{}
		for _, reward := range exist {
			key := fmt.Sprintf("%d-%d-%s", reward.AddressID, reward.ValidatorID, reward.Role)
			existRewardsMap[key] = reward
		}

		var aggregated []*models.AggregatedReward
		for _, userRewards := range rewardsMap {
			key := fmt.Sprintf("%d-%d-%s", userRewards[0].AddressID, userRewards[0].ValidatorID, userRewards[0].Role)
			total := big.NewInt(0)
			for _, r := range userRewards {
				amount, _ := big.NewInt(0).SetString(r.Amount, 10)
				total.Add(total, amount)
			}

			if existRewardsMap[key] != nil {
				existAmount, _ := big.NewInt(0).SetString(existRewardsMap[key].Amount, 10)
				total.Add(total, existAmount)
			}

			aggregated = append(aggregated, &models.AggregatedReward{
				FromBlockID: userRewards[0].BlockID,
				ToBlockID:   userRewards[0].BlockID,
				AddressID:   uint64(userRewards[0].AddressID),
				ValidatorID: userRewards[0].ValidatorID,
				Role:        userRewards[0].Role,
				Amount:      total.String(),
				TimeID:      timeId,
			})
		}

		err = s.repository.SaveAggregatedRewards(aggregated)
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
	s.GetSaveRewardsJobChannel() <- rewards
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

func (s *Service) handleRewardEvent(blockHeight uint64, e *api.RewardEvent) (*models.Reward, error) {
	addressId, err := s.addressRepository.FindId(helpers.RemovePrefix(e.Address))
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"address": e.Address,
		}).Error(err)
		return nil, err
	}

	validatorId, err := s.validatorRepository.FindIdByPk(helpers.RemovePrefix(e.GetPublicKey()))
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"public_key": e.GetPublicKey(),
		}).Error(err)
		return nil, err
	}

	return &models.Reward{
		BlockID:     blockHeight,
		Role:        e.Role,
		Amount:      e.Amount,
		AddressID:   addressId,
		ValidatorID: uint64(validatorId),
	}, nil
}
