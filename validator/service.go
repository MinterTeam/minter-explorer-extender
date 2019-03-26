package validator

import (
	"github.com/MinterTeam/minter-explorer-extender/address"
	"github.com/MinterTeam/minter-explorer-extender/coin"
	"github.com/MinterTeam/minter-explorer-tools/helpers"
	"github.com/MinterTeam/minter-explorer-tools/models"
	"github.com/daniildulin/minter-node-api"
	"github.com/daniildulin/minter-node-api/responses"
	"github.com/sirupsen/logrus"
	"strconv"
	"time"
)

type Service struct {
	nodeApi             *minter_node_api.MinterNodeApi
	repository          *Repository
	addressRepository   *address.Repository
	coinRepository      *coin.Repository
	jobUpdateValidators chan uint64
	jobUpdateStakes     chan uint64
	logger              *logrus.Entry
}

func NewService(nodeApi *minter_node_api.MinterNodeApi, repository *Repository, addressRepository *address.Repository,
	coinRepository *coin.Repository, logger *logrus.Entry) *Service {
	return &Service{
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
		helpers.HandleError(err)
		var (
			vl           []*models.Validator
			addressesMap = make(map[string]struct{})
		)

		// Collect all PubKey's and addresses for save it before
		for _, vlr := range resp.Result {
			vl = append(vl, &models.Validator{PublicKey: helpers.RemovePrefix(vlr.PubKey)})
			addressesMap[helpers.RemovePrefix(vlr.RewardAddress)] = struct{}{}
			addressesMap[helpers.RemovePrefix(vlr.OwnerAddress)] = struct{}{}
		}

		err = s.repository.SaveAllIfNotExist(vl)
		if err != nil {
			s.logger.Error(err)
		}

		err = s.addressRepository.SaveFromMapIfNotExists(addressesMap)
		if err != nil {
			s.logger.Error(err)
		}

		vl = make([]*models.Validator, len(resp.Result))
		for _, vlr := range resp.Result {
			id, err := s.repository.FindIdByPkOrCreate(helpers.RemovePrefix(vlr.PubKey))
			s.logger.Error(err)
			helpers.HandleError(err)
			updateAt := time.Now()
			commission, err := strconv.ParseUint(vlr.Commission, 10, 64)
			s.logger.Error(err)
			helpers.HandleError(err)
			rewardAddressID, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(vlr.RewardAddress))
			s.logger.Error(err)
			helpers.HandleError(err)
			ownerAddressID, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(vlr.OwnerAddress))
			s.logger.Error(err)
			helpers.HandleError(err)
			vl = append(vl, &models.Validator{
				ID:              id,
				Status:          &vlr.Status,
				TotalStake:      &vlr.TotalStake,
				UpdateAt:        &updateAt,
				Commission:      &commission,
				RewardAddressID: &rewardAddressID,
				OwnerAddressID:  &ownerAddressID,
			})
		}
		err = s.repository.ResetAllStatuses()
		s.logger.Error(err)
		helpers.HandleError(err)
		err = s.repository.UpdateAll(vl)
		s.logger.Error(err)
		helpers.HandleError(err)
	}
}

func (s *Service) UpdateStakesWorker(jobs <-chan uint64) {
	for height := range jobs {
		resp, err := s.nodeApi.GetCandidates(height, true)
		if err != nil {
			s.logger.Error(err)
		}
		helpers.HandleError(err)
		var (
			stakes       []*models.Stake
			validatorIds []uint64
			vl           []*models.Validator
			addressesMap = make(map[string]struct{})
		)

		// Collect all PubKey's and addresses for save it before
		for _, vlr := range resp.Result {
			vl = append(vl, &models.Validator{PublicKey: helpers.RemovePrefix(vlr.PubKey)})
			addressesMap[helpers.RemovePrefix(vlr.RewardAddress)] = struct{}{}
			addressesMap[helpers.RemovePrefix(vlr.OwnerAddress)] = struct{}{}
			for _, stake := range vlr.Stakes {
				addressesMap[helpers.RemovePrefix(stake.Owner)] = struct{}{}
			}
		}

		err = s.repository.SaveAllIfNotExist(vl)
		if err != nil {
			s.logger.Error(err)
		}

		err = s.addressRepository.SaveFromMapIfNotExists(addressesMap)
		if err != nil {
			s.logger.Error(err)
		}

		for _, vlr := range resp.Result {
			id, err := s.repository.FindIdByPkOrCreate(helpers.RemovePrefix(vlr.PubKey))
			if err != nil {
				s.logger.Error(err)
			}
			helpers.HandleError(err)
			validatorIds = append(validatorIds, id)
			for _, stake := range vlr.Stakes {
				ownerAddressID, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(stake.Owner))
				if err != nil {
					s.logger.Error(err)
				}
				helpers.HandleError(err)
				coinID, err := s.coinRepository.FindIdBySymbol(stake.Coin)
				if err != nil {
					s.logger.Error(err)
				}
				helpers.HandleError(err)
				stakes = append(stakes, &models.Stake{
					ValidatorID:    id,
					OwnerAddressID: ownerAddressID,
					CoinID:         coinID,
					Value:          stake.Value,
					BipValue:       stake.BipValue,
				})
			}
		}
		err = s.repository.DeleteStakesByValidatorIds(validatorIds)
		if err != nil {
			s.logger.Error(err)
		}
		helpers.HandleError(err)
		err = s.repository.SaveAllStakes(stakes)
		if err != nil {
			s.logger.Error(err)
		}
		helpers.HandleError(err)
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
