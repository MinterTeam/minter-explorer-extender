package validator

import (
	"github.com/MinterTeam/minter-explorer-extender/address"
	"github.com/MinterTeam/minter-explorer-extender/coin"
	"github.com/MinterTeam/minter-explorer-tools/helpers"
	"github.com/MinterTeam/minter-explorer-tools/models"
	"github.com/MinterTeam/minter-node-go-api"
	"github.com/MinterTeam/minter-node-go-api/responses"
	"github.com/sirupsen/logrus"
	"math"
	"strconv"
	"time"
)

type Service struct {
	env                 *models.ExtenderEnvironment
	nodeApi             *minter_node_go_api.MinterNodeApi
	repository          *Repository
	addressRepository   *address.Repository
	coinRepository      *coin.Repository
	jobUpdateValidators chan uint64
	jobUpdateStakes     chan uint64
	logger              *logrus.Entry
}

func NewService(env *models.ExtenderEnvironment, nodeApi *minter_node_go_api.MinterNodeApi, repository *Repository,
	addressRepository *address.Repository, coinRepository *coin.Repository, logger *logrus.Entry) *Service {
	return &Service{
		env:                 env,
		nodeApi:             nodeApi,
		repository:          repository,
		addressRepository:   addressRepository,
		coinRepository:      coinRepository,
		logger:              logger,
		jobUpdateValidators: make(chan uint64, 1),
		jobUpdateStakes:     make(chan uint64, 1),
	}
}

func (s *Service) GetUpdateValidatorsJobChannel() chan uint64 {
	return s.jobUpdateValidators
}

func (s *Service) GetUpdateStakesJobChannel() chan uint64 {
	return s.jobUpdateStakes
}

func (s *Service) UpdateValidatorsWorker(jobs <-chan uint64) {
	for height := range jobs {
		resp, err := s.nodeApi.GetCandidates(height, false)
		if err != nil {
			s.logger.Error(err)
		}

		if len(resp.Result) > 0 {
			var (
				validators   = make([]*models.Validator, len(resp.Result))
				addressesMap = make(map[string]struct{})
			)

			// Collect all PubKey's and addresses for save it before
			for i, vlr := range resp.Result {
				validators[i] = &models.Validator{PublicKey: helpers.RemovePrefix(vlr.PubKey)}
				addressesMap[helpers.RemovePrefix(vlr.RewardAddress)] = struct{}{}
				addressesMap[helpers.RemovePrefix(vlr.OwnerAddress)] = struct{}{}
			}

			err = s.repository.SaveAllIfNotExist(validators)
			if err != nil {
				s.logger.Error(err)
			}

			err = s.addressRepository.SaveFromMapIfNotExists(addressesMap)
			if err != nil {
				s.logger.Error(err)
			}

			for i, validator := range resp.Result {
				updateAt := time.Now()
				status := validator.Status
				totalStake := validator.TotalStake

				id, err := s.repository.FindIdByPkOrCreate(helpers.RemovePrefix(validator.PubKey))
				if err != nil {
					s.logger.Error(err)
					continue
				}
				commission, err := strconv.ParseUint(validator.Commission, 10, 64)
				if err != nil {
					s.logger.Error(err)
					continue
				}
				rewardAddressID, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(validator.RewardAddress))
				if err != nil {
					s.logger.Error(err)
					continue
				}
				ownerAddressID, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(validator.OwnerAddress))
				if err != nil {
					s.logger.Error(err)
					continue
				}
				validators[i] = &models.Validator{
					ID:              id,
					Status:          &status,
					TotalStake:      &totalStake,
					UpdateAt:        &updateAt,
					Commission:      &commission,
					RewardAddressID: &rewardAddressID,
					OwnerAddressID:  &ownerAddressID,
				}
			}
			err = s.repository.ResetAllStatuses()
			if err != nil {
				s.logger.Error(err)
			}
			err = s.repository.UpdateAll(validators)
			if err != nil {
				s.logger.Error(err)
			}
		}
	}
}

func (s *Service) UpdateStakesWorker(jobs <-chan uint64) {
	for height := range jobs {
		resp, err := s.nodeApi.GetCandidates(height, true)
		if err != nil {
			s.logger.Error(err)
		}
		var (
			stakes       []*models.Stake
			validatorIds = make([]uint64, len(resp.Result))
			validators   = make([]*models.Validator, len(resp.Result))
			addressesMap = make(map[string]struct{})
		)

		// Collect all PubKey's and addresses for save it before
		for i, vlr := range resp.Result {
			validators[i] = &models.Validator{PublicKey: helpers.RemovePrefix(vlr.PubKey)}
			addressesMap[helpers.RemovePrefix(vlr.RewardAddress)] = struct{}{}
			addressesMap[helpers.RemovePrefix(vlr.OwnerAddress)] = struct{}{}
			for _, stake := range vlr.Stakes {
				addressesMap[helpers.RemovePrefix(stake.Owner)] = struct{}{}
			}
		}

		err = s.repository.SaveAllIfNotExist(validators)
		if err != nil {
			s.logger.Error(err)
		}

		err = s.addressRepository.SaveFromMapIfNotExists(addressesMap)
		if err != nil {
			s.logger.Error(err)
		}

		for i, vlr := range resp.Result {
			id, err := s.repository.FindIdByPkOrCreate(helpers.RemovePrefix(vlr.PubKey))
			if err != nil {
				s.logger.Error(err)
				continue
			}
			validatorIds[i] = id
			for _, stake := range vlr.Stakes {
				ownerAddressID, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(stake.Owner))
				if err != nil {
					s.logger.Error(err)
					continue
				}
				coinID, err := s.coinRepository.FindIdBySymbol(stake.Coin)
				if err != nil {
					s.logger.Error(err)
					continue
				}
				stakes = append(stakes, &models.Stake{
					ValidatorID:    id,
					OwnerAddressID: ownerAddressID,
					CoinID:         coinID,
					Value:          stake.Value,
					BipValue:       stake.BipValue,
				})
			}
		}

		chunksCount := int(math.Ceil(float64(len(stakes)) / float64(s.env.StakeChunkSize)))
		for i := 0; i < chunksCount; i++ {
			start := s.env.StakeChunkSize * i
			end := start + s.env.StakeChunkSize
			if end > len(stakes) {
				end = len(stakes)
			}
			err = s.repository.SaveAllStakes(stakes[start:end])
			if err != nil {
				s.logger.Error(err)
				panic(err)
			}
		}

		stakesId := make([]uint64, len(stakes))
		for i, stake := range stakes {
			stakesId[i] = stake.ID
		}
		err = s.repository.DeleteStakesNotInListIds(stakesId)
		if err != nil {
			s.logger.Error(err)
		}
	}
}

//Get validators PK from response and store it to validators table if not exist
func (s *Service) HandleBlockResponse(response *responses.BlockResponse) ([]*models.Validator, error) {
	var validators []*models.Validator
	for _, v := range response.Result.Validators {
		validators = append(validators, &models.Validator{PublicKey: helpers.RemovePrefix(v.PubKey)})
	}
	err := s.repository.SaveAllIfNotExist(validators)
	if err != nil {
		s.logger.Error(err)
		return nil, err
	}
	return validators, err
}

func (s *Service) HandleCandidateResponse(response *responses.CandidateResponse) (*models.Validator, []*models.Stake, error) {
	validator := new(models.Validator)
	validator.Status = &response.Result.Status
	validator.TotalStake = &response.Result.TotalStake
	commission, err := strconv.ParseUint(response.Result.Commission, 10, 64)
	if err != nil {
		s.logger.Error(err)
		return nil, nil, err
	}
	validator.Commission = &commission
	createdAtBlockID, err := strconv.ParseUint(response.Result.CreatedAtBlock, 10, 64)
	if err != nil {
		s.logger.Error(err)
		return nil, nil, err
	}
	validator.CreatedAtBlockID = &createdAtBlockID
	ownerAddressID, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(response.Result.OwnerAddress))
	if err != nil {
		s.logger.Error(err)
		return nil, nil, err
	}
	validator.OwnerAddressID = &ownerAddressID
	rewardAddressID, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(response.Result.RewardAddress))
	if err != nil {
		s.logger.Error(err)
		return nil, nil, err
	}
	validator.RewardAddressID = &rewardAddressID
	validator.PublicKey = helpers.RemovePrefix(response.Result.PubKey)
	validatorID, err := s.repository.FindIdByPk(validator.PublicKey)
	if err != nil {
		s.logger.Error(err)
		return nil, nil, err
	}
	validator.ID = validatorID
	now := time.Now()
	validator.UpdateAt = &now

	stakes, err := s.GetStakesFromCandidateResponse(response)
	if err != nil {
		s.logger.Error(err)
		return nil, nil, err
	}

	return validator, stakes, nil
}

func (s *Service) GetStakesFromCandidateResponse(response *responses.CandidateResponse) ([]*models.Stake, error) {
	var stakes []*models.Stake
	validatorID, err := s.repository.FindIdByPk(helpers.RemovePrefix(response.Result.PubKey))
	if err != nil {
		s.logger.Error(err)
		return nil, err
	}
	for _, stake := range response.Result.Stakes {
		ownerAddressID, err := s.addressRepository.FindId(helpers.RemovePrefix(stake.Owner))
		if err != nil {
			s.logger.Error(err)
			return nil, err
		}
		coinID, err := s.coinRepository.FindIdBySymbol(stake.Coin)
		if err != nil {
			s.logger.Error(err)
			return nil, err
		}
		stakes = append(stakes, &models.Stake{
			CoinID:         coinID,
			Value:          stake.Value,
			ValidatorID:    validatorID,
			BipValue:       stake.BipValue,
			OwnerAddressID: ownerAddressID,
		})
	}
	return stakes, nil
}
