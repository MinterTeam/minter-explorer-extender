package validator

import (
	"fmt"
	"github.com/MinterTeam/minter-explorer-extender/v2/address"
	"github.com/MinterTeam/minter-explorer-extender/v2/coin"
	"github.com/MinterTeam/minter-explorer-extender/v2/env"
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/MinterTeam/minter-explorer-tools/v4/helpers"
	"github.com/MinterTeam/minter-go-sdk/v2/api/grpc_client"
	"github.com/MinterTeam/minter-go-sdk/v2/transaction"
	"github.com/MinterTeam/node-grpc-gateway/api_pb"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/anypb"
	"math"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	env                 *env.ExtenderEnvironment
	nodeApi             *grpc_client.Client
	repository          *Repository
	addressRepository   *address.Repository
	coinRepository      *coin.Repository
	jobUpdateValidators chan int
	jobUpdateStakes     chan int
	jobUpdateWaitList   chan *models.Transaction
	jobUnbondSaver      chan *models.Transaction
	logger              *logrus.Entry
}

func NewService(env *env.ExtenderEnvironment, nodeApi *grpc_client.Client, repository *Repository, addressRepository *address.Repository, coinRepository *coin.Repository, logger *logrus.Entry) *Service {
	return &Service{
		env:                 env,
		nodeApi:             nodeApi,
		repository:          repository,
		addressRepository:   addressRepository,
		coinRepository:      coinRepository,
		logger:              logger,
		jobUpdateValidators: make(chan int, 1),
		jobUpdateStakes:     make(chan int, 1),
		jobUpdateWaitList:   make(chan *models.Transaction, 1),
		jobUnbondSaver:      make(chan *models.Transaction, 1),
	}
}

func (s *Service) GetUpdateValidatorsJobChannel() chan int {
	return s.jobUpdateValidators
}

func (s *Service) GetUpdateStakesJobChannel() chan int {
	return s.jobUpdateStakes
}
func (s *Service) GetUpdateWaitListJobChannel() chan *models.Transaction {
	return s.jobUpdateWaitList
}
func (s *Service) GetUnbondSaverJobChannel() chan *models.Transaction {
	return s.jobUnbondSaver
}

func (s *Service) UnbondSaverWorker(data <-chan *models.Transaction) {
	for tx := range data {
		txData := new(api_pb.UnbondData)
		if err := tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
			s.logger.Error(err)
			continue
		}

		vId, err := s.repository.FindIdByPk(helpers.RemovePrefix(txData.PubKey))
		if err != nil {
			s.logger.Error(err)
			continue
		}

		coinId, err := strconv.ParseUint(txData.Coin.Id, 10, 64)
		if err != nil {
			s.logger.Error(err)
			continue
		}

		unbond := &models.Unbond{
			BlockId:     uint(tx.BlockID),
			AddressId:   uint(tx.FromAddressID),
			CoinId:      uint(coinId),
			ValidatorId: vId,
			Value:       txData.Value,
		}

		err = s.repository.AddUnbond(unbond)

	}
}

func (s *Service) UpdateWaitListWorker(data <-chan *models.Transaction) {
	for tx := range data {

		var adr, pk string

		adr, err := s.addressRepository.FindById(uint(tx.FromAddressID))
		if err != nil {
			s.logger.Error(err)
			continue
		}

		if transaction.Type(tx.Type) == transaction.TypeUnbond {
			txData := new(api_pb.UnbondData)
			if err = tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
				s.logger.Error(err)
				continue
			}
			pk = txData.PubKey
		}
		if transaction.Type(tx.Type) == transaction.TypeDelegate {
			txData := new(api_pb.DelegateData)
			if err = tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
				s.logger.Error(err)
				continue
			}
			pk = txData.PubKey
		}

		if err = s.UpdateWaitList(adr, pk); err != nil {
			s.logger.Error(err)
		}
	}
}

func (s *Service) UpdateValidatorsWorker(jobs <-chan int) {
	for height := range jobs {
		resp, err := s.nodeApi.Candidates(false, height)
		if err != nil {
			s.logger.WithField("Block", height).Error(err)
		}

		if len(resp.Candidates) > 0 {
			var (
				validators      []*models.Validator
				validatorsPkMap = make(map[string]struct{})
				addressesMap    = make(map[string]struct{})
			)

			// Collect all PubKey's and addresses for save it before
			for _, vlr := range resp.Candidates {
				validatorsPkMap[helpers.RemovePrefix(vlr.PublicKey)] = struct{}{}
				addressesMap[helpers.RemovePrefix(vlr.RewardAddress)] = struct{}{}
				addressesMap[helpers.RemovePrefix(vlr.ControlAddress)] = struct{}{}
			}

			err = s.repository.SaveAllIfNotExist(validatorsPkMap)
			if err != nil {
				s.logger.Error(err)
			}

			err = s.addressRepository.SaveFromMapIfNotExists(addressesMap)
			if err != nil {
				s.logger.Error(err)
			}

			for _, validator := range resp.Candidates {
				updateAt := time.Now()
				totalStake := validator.TotalStake

				sts, err := strconv.ParseUint(validator.Status, 10, 64)
				if err != nil {
					s.logger.Error(err)
					continue
				}
				status := uint8(sts)

				id, err := s.repository.FindIdByPkOrCreate(helpers.RemovePrefix(validator.PublicKey))
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
				validators = append(validators, &models.Validator{
					ID:              id,
					Status:          &status,
					TotalStake:      &totalStake,
					UpdateAt:        &updateAt,
					Commission:      &commission,
					OwnerAddressID:  &ownerAddressID,
					RewardAddressID: &rewardAddressID,
				})
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

func (s *Service) UpdateStakesWorker(jobs <-chan int) {
	for height := range jobs {
		resp, err := s.nodeApi.Candidates(true, height)
		if err != nil {
			s.logger.WithField("Block", height).Error(err)
		}
		var (
			stakes       []*models.Stake
			validatorIds = make([]uint64, len(resp.Candidates))
			addressesMap = make(map[string]struct{})
		)

		validatorsPkMap := make(map[string]struct{})
		// Collect all PubKey's and addresses for save it before
		for _, vlr := range resp.Candidates {
			validatorsPkMap[helpers.RemovePrefix(vlr.PublicKey)] = struct{}{}
			addressesMap[helpers.RemovePrefix(vlr.RewardAddress)] = struct{}{}
			addressesMap[helpers.RemovePrefix(vlr.OwnerAddress)] = struct{}{}
			for _, stake := range vlr.Stakes {
				addressesMap[helpers.RemovePrefix(stake.Owner)] = struct{}{}
			}
		}

		err = s.repository.SaveAllIfNotExist(validatorsPkMap)
		if err != nil {
			s.logger.Error(err)
		}

		err = s.addressRepository.SaveFromMapIfNotExists(addressesMap)
		if err != nil {
			s.logger.Error(err)
		}

		for i, vlr := range resp.Candidates {
			id, err := s.repository.FindIdByPk(helpers.RemovePrefix(vlr.PublicKey))
			if err != nil {
				s.logger.WithField("pk", vlr.PublicKey).Error(err)
				continue
			}
			validatorIds[i] = uint64(id)
			for _, stake := range vlr.Stakes {
				ownerAddressID, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(stake.Owner))
				if err != nil {
					s.logger.Error(err)
					continue
				}
				coinID, err := strconv.ParseUint(stake.Coin.Id, 10, 64)
				if err != nil {
					s.logger.Error(err)
					continue
				}
				stakes = append(stakes, &models.Stake{
					ValidatorID:    id,
					OwnerAddressID: ownerAddressID,
					CoinID:         uint(coinID),
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
				s.logger.Fatal(err)
			}
		}

		stakesId := make([]uint64, len(stakes))
		for i, stake := range stakes {
			stakesId[i] = uint64(stake.ID)
		}
		err = s.repository.DeleteStakesNotInListIds(stakesId)
		if err != nil {
			s.logger.Error(err)
		}
	}
}

//Get validators PK from response and store it to validators table if not exist
func (s *Service) HandleBlockResponse(response *api_pb.BlockResponse) error {
	validatorsPkMap := make(map[string]struct{})

	for _, v := range response.Validators {
		validatorsPkMap[helpers.RemovePrefix(v.PublicKey)] = struct{}{}
	}

	err := s.repository.SaveAllIfNotExist(validatorsPkMap)
	if err != nil {
		return err
	}

	for _, tx := range response.Transactions {

		txType, err := strconv.ParseUint(tx.Type, 10, 64)
		if err != nil {
			return err
		}

		if transaction.Type(txType) == transaction.TypeDeclareCandidacy {
			txData := new(api_pb.DeclareCandidacyData)
			if err := tx.Data.UnmarshalTo(txData); err != nil {
				return err
			}

			_, err := s.repository.FindIdByPkOrCreate(helpers.RemovePrefix(txData.PubKey))
			if err != nil {
				return err
			}
		}

		if transaction.Type(txType) == transaction.TypeEditCandidatePublicKey {
			txData := new(api_pb.EditCandidatePublicKeyData)
			if err := tx.Data.UnmarshalTo(txData); err != nil {
				return err
			}

			vId, err := s.repository.FindIdByPk(helpers.RemovePrefix(txData.PubKey))
			if err != nil {
				return err
			}

			v, err := s.repository.GetById(vId)
			if err != nil {
				return err
			}

			err = s.repository.AddPk(vId, helpers.RemovePrefix(txData.NewPubKey))
			if err != nil {
				return err
			}

			v.PublicKey = helpers.RemovePrefix(txData.NewPubKey)
			err = s.repository.Update(v)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *Service) HandleCandidateResponse(response *api_pb.CandidateResponse) (*models.Validator, []*models.Stake, error) {
	validator := new(models.Validator)
	sts, err := strconv.ParseUint(response.Status, 10, 64)
	status := uint8(sts)
	validator.Status = &status
	validator.TotalStake = &response.TotalStake
	commission, err := strconv.ParseUint(response.Commission, 10, 64)
	if err != nil {
		s.logger.Error(err)
		return nil, nil, err
	}
	validator.Commission = &commission

	validator.PublicKey = helpers.RemovePrefix(response.PublicKey)

	ownerAddressID, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(response.OwnerAddress))
	if err != nil {
		return nil, nil, err
	}
	validator.OwnerAddressID = &ownerAddressID
	rewardAddressID, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(response.RewardAddress))
	if err != nil {
		return nil, nil, err
	}
	validator.RewardAddressID = &rewardAddressID

	controlAddressID, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(response.ControlAddress))
	if err != nil {
		return nil, nil, err
	}
	validator.ControlAddressID = &controlAddressID

	validatorID, err := s.repository.FindIdByPk(helpers.RemovePrefix(response.PublicKey))
	if err != nil {
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

func (s *Service) GetStakesFromCandidateResponse(response *api_pb.CandidateResponse) ([]*models.Stake, error) {
	var stakes []*models.Stake
	validatorID, err := s.repository.FindIdByPk(helpers.RemovePrefix(response.PublicKey))
	if err != nil {
		s.logger.Error(err)
		return nil, err
	}
	for _, stake := range response.Stakes {
		ownerAddressID, err := s.addressRepository.FindId(helpers.RemovePrefix(stake.Owner))
		if err != nil {
			s.logger.Error(err)
			return nil, err
		}
		coinID, err := strconv.ParseUint(stake.Coin.Id, 10, 64)
		if err != nil {
			s.logger.Error(err)
			return nil, err
		}
		stakes = append(stakes, &models.Stake{
			CoinID:         uint(coinID),
			Value:          stake.Value,
			ValidatorID:    validatorID,
			BipValue:       stake.BipValue,
			OwnerAddressID: ownerAddressID,
		})
	}
	return stakes, nil
}

func (s *Service) UpdateWaitList(adr, pk string) error {
	var err error
	var addressId uint
	var data *api_pb.WaitListResponse

	strRune := []rune(adr)
	prefix := string(strRune[0:2])

	if strings.ToLower(prefix) == "mx" {
		data, err = s.nodeApi.WaitList(pk, adr)
	} else {
		data, err = s.nodeApi.WaitList(pk, fmt.Sprintf("Mx%s", adr))
	}
	if err != nil {
		return err
	}

	if strings.ToLower(prefix) == "mx" {
		addressId, err = s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(adr))
	} else {
		addressId, err = s.addressRepository.FindIdOrCreate(adr)
	}
	if err != nil {
		return err
	}

	vId, err := s.repository.FindIdByPk(helpers.RemovePrefix(pk))
	if err != nil {
		return err
	}

	if len(data.List) == 0 {
		return s.repository.RemoveFromWaitList(addressId, vId)
	}

	var existCoins []uint64

	for _, item := range data.List {
		coinId, err := strconv.ParseUint(item.Coin.Id, 10, 64)
		if err != nil {
			s.logger.Error(err)
			continue
		}

		existCoins = append(existCoins, coinId)

		stk := &models.Stake{
			OwnerAddressID: addressId,
			CoinID:         uint(coinId),
			ValidatorID:    vId,
			Value:          item.Value,
			IsKicked:       true,
		}

		err = s.repository.UpdateStake(stk)
		if err != nil {
			s.logger.Error(err)
			continue
		}
	}

	return s.repository.DeleteFromWaitList(addressId, vId, existCoins)
}
