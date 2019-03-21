package validator

import (
	"errors"
	"github.com/MinterTeam/minter-explorer-extender/address"
	"github.com/MinterTeam/minter-explorer-extender/coin"
	"github.com/MinterTeam/minter-explorer-tools/helpers"
	"github.com/MinterTeam/minter-explorer-tools/models"
	"github.com/daniildulin/minter-node-api"
	"github.com/daniildulin/minter-node-api/responses"
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
}

func NewService(nodeApi *minter_node_api.MinterNodeApi, repository *Repository, addressRepository *address.Repository,
	coinRepository *coin.Repository) *Service {
	return &Service{
		nodeApi:             nodeApi,
		repository:          repository,
		addressRepository:   addressRepository,
		coinRepository:      coinRepository,
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
		resp, err := s.nodeApi.GetCandidates(height)
		helpers.HandleError(err)

		err = s.repository.ResetAllStatuses()
		helpers.HandleError(err)

		for _, vlr := range resp.Result {
			id, err := s.repository.FindIdByPkOrCreate(helpers.RemovePrefix(vlr.PubKey))
			helpers.HandleError(err)
			updateAt := time.Now()
			commission, err := strconv.ParseUint(vlr.Commission, 10, 64)
			helpers.HandleError(err)
			rewardAddressID, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(vlr.RewardAddress))
			helpers.HandleError(err)
			ownerAddressID, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(vlr.OwnerAddress))
			helpers.HandleError(err)
			err = s.repository.Update(&models.Validator{
				ID:              id,
				PublicKey:       helpers.RemovePrefix(vlr.PubKey),
				Status:          &vlr.Status,
				TotalStake:      &vlr.TotalStake,
				UpdateAt:        &updateAt,
				Commission:      &commission,
				RewardAddressID: &rewardAddressID,
				OwnerAddressID:  &ownerAddressID,
			})
			helpers.HandleError(err)
		}
	}
}

func (s *Service) UpdateStakesWorker(jobs <-chan uint64) {
	//TODO: implement
}

//Get validators PK from response and store it to validators table if not exist
func (s *Service) HandleBlockResponse(response *responses.BlockResponse) ([]*models.Validator, error) {
	var validators []*models.Validator
	for _, v := range response.Result.Validators {
		validators = append(validators, &models.Validator{PublicKey: helpers.RemovePrefix(v.PubKey)})
	}
	err := s.repository.SaveAllIfNotExist(validators)
	if err != nil {
		return nil, err
	}
	return validators, err
}

func (s *Service) UpdateValidatorsInfoAndStakes(response *responses.CandidateResponse) error {
	validator, stakes, err := s.HandleCandidateResponse(response)
	if err != nil {
		return err
	}
	if validator.ID == 0 {
		return errors.New("validator does't exists")
	}
	err = s.repository.Update(validator)
	if err != nil {
		return err
	}
	return s.repository.UpdateStakesByValidatorId(validator.ID, stakes)
}

func (s *Service) HandleCandidateResponse(response *responses.CandidateResponse) (*models.Validator, []*models.Stake, error) {
	validator := new(models.Validator)
	validator.Status = &response.Result.Status
	validator.TotalStake = &response.Result.TotalStake
	commission, err := strconv.ParseUint(response.Result.Commission, 10, 64)
	if err != nil {
		return nil, nil, err
	}
	validator.Commission = &commission
	createdAtBlockID, err := strconv.ParseUint(response.Result.CreatedAtBlock, 10, 64)
	if err != nil {
		return nil, nil, err
	}
	validator.CreatedAtBlockID = &createdAtBlockID
	ownerAddressID, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(response.Result.OwnerAddress))
	if err != nil {
		return nil, nil, err
	}
	validator.OwnerAddressID = &ownerAddressID
	rewardAddressID, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(response.Result.RewardAddress))
	if err != nil {
		return nil, nil, err
	}
	validator.RewardAddressID = &rewardAddressID
	validator.PublicKey = helpers.RemovePrefix(response.Result.PubKey)
	validatorID, err := s.repository.FindIdByPk(validator.PublicKey)
	if err != nil {
		return nil, nil, err
	}
	validator.ID = validatorID
	now := time.Now()
	validator.UpdateAt = &now

	stakes, err := s.GetStakesFromCandidateResponse(response)
	if err != nil {
		return nil, nil, err
	}

	return validator, stakes, nil
}

func (s *Service) GetStakesFromCandidateResponse(response *responses.CandidateResponse) ([]*models.Stake, error) {
	var stakes []*models.Stake
	validatorID, err := s.repository.FindIdByPk(helpers.RemovePrefix(response.Result.PubKey))
	if err != nil {
		return nil, err
	}
	for _, stake := range response.Result.Stakes {
		ownerAddressID, err := s.addressRepository.FindId(helpers.RemovePrefix(stake.Owner))
		if err != nil {
			return nil, err
		}
		coinID, err := s.coinRepository.FindIdBySymbol(stake.Coin)
		if err != nil {
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
