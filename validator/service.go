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
	"math/big"
	"sync/atomic"
	"time"
)

const (
	UnbondBlockCount           = 518400
	UnbondBlockCountTestnet    = 532
	MoveStakeBlockCount        = 134400
	MoveStakeBlockCountTestnet = 177
	UpdateTimoutInBlocks       = 720
	ChasingModDiff             = 121
)

type Service struct {
	env                 *env.ExtenderEnvironment
	nodeApi             *grpc_client.Client
	repository          *Repository
	addressRepository   *address.Repository
	coinRepository      *coin.Repository
	jobUpdateValidators chan uint64
	jobClearChannel     chan uint64
	jobUnbondSaver      chan *models.Transaction
	jobMoveStake        chan *api_pb.TransactionResponse
	logger              *logrus.Entry
	//jobUpdateStakes     chan uint64
	//jobUpdateWaitList chan *models.Transaction
}

func NewService(env *env.ExtenderEnvironment, nodeApi *grpc_client.Client, repository *Repository, addressRepository *address.Repository, coinRepository *coin.Repository, logger *logrus.Entry) *Service {
	chasingMode := atomic.Value{}
	chasingMode.Store(false)

	return &Service{
		env:                 env,
		nodeApi:             nodeApi,
		repository:          repository,
		addressRepository:   addressRepository,
		coinRepository:      coinRepository,
		logger:              logger,
		jobUpdateValidators: make(chan uint64, 1),
		jobClearChannel:     make(chan uint64, 1),
		jobUnbondSaver:      make(chan *models.Transaction, 1),
		jobMoveStake:        make(chan *api_pb.TransactionResponse, 1),
		//jobUpdateWaitList: make(chan *models.Transaction, 1),
		//jobUpdateStakes:     make(chan uint64, 1),
	}
}

func (s *Service) GetClearJobChannel() chan uint64 {
	return s.jobClearChannel
}

func (s *Service) GetUpdateValidatorsJobChannel() chan uint64 {
	return s.jobUpdateValidators
}

//func (s *Service) GetUpdateStakesJobChannel() chan uint64 {
//	return s.jobUpdateStakes
//}
//func (s *Service) GetUpdateWaitListJobChannel() chan *models.Transaction {
//	return s.jobUpdateWaitList
//}
func (s *Service) GetUnbondSaverJobChannel() chan *models.Transaction {
	return s.jobUnbondSaver
}
func (s *Service) GetMoveStakeJobChannel() chan *api_pb.TransactionResponse {
	return s.jobMoveStake
}

func (s *Service) ClearMoveStakeAndUnbondWorker(height <-chan uint64) {
	for h := range height {

		err := s.repository.DeleteOldUnbonds(h)
		if err != nil {
			s.logger.Error(err)
		}

		err = s.repository.DeleteOldMovedStakes(h)
		if err != nil {
			s.logger.Error(err)
		}

	}
}

func (s *Service) MoveStakeWorker(data <-chan *api_pb.TransactionResponse) {
	for tx := range data {
		txData := new(api_pb.MoveStakeData)
		if err := tx.GetData().UnmarshalTo(txData); err != nil {
			s.logger.Error(err)
			continue
		}

		fromId, err := s.repository.FindIdByPk(helpers.RemovePrefix(txData.FromPubKey))
		if err != nil {
			s.logger.Error(err)
			continue
		}
		toId, err := s.repository.FindIdByPk(helpers.RemovePrefix(txData.ToPubKey))
		if err != nil {
			s.logger.Error(err)
			continue
		}

		aId, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(tx.From))
		if err != nil {
			s.logger.Error(err)
			continue
		}

		ms := &models.MovedStake{
			BlockId:         tx.Height + s.GetMoveStakeBlockCount(),
			AddressId:       uint64(aId),
			CoinId:          txData.Coin.Id,
			FromValidatorId: uint64(fromId),
			ToValidatorId:   uint64(toId),
			Value:           txData.Value,
		}

		//if err = s.UpdateWaitList(tx.From, txData.FromPubKey); err != nil {
		//	s.logger.Error(err)
		//}

		err = s.repository.MoveStake(ms)
		if err != nil {
			s.logger.Error(err)
		}

		stk, err := s.repository.GetStake(uint64(aId), uint64(fromId), txData.Coin.Id)

		currentVal, ok := big.NewInt(0).SetString(stk.Value, 10)
		if !ok {
			s.logger.Error("can't convert to big.Int")
			continue
		}
		moveVal, ok := big.NewInt(0).SetString(stk.Value, 10)
		if !ok {
			s.logger.Error("can't convert to big.Int")
			continue
		}

		newVal := currentVal.Sub(currentVal, moveVal)

		if newVal.Cmp(big.NewInt(0)) <= 0 {
			if err = s.repository.DeleteStake(uint64(aId), uint64(fromId), txData.Coin.Id); err != nil {
				s.logger.Error(err)
			}
		} else {
			stk.Value = newVal.String()
			if err = s.repository.UpdateStake(stk); err != nil {
				s.logger.Error(err)
			}
		}

	}
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

		unbond := &models.Unbond{
			BlockId:     uint(tx.BlockID + s.GetUnbondBlockCount()),
			AddressId:   uint(tx.FromAddressID),
			CoinId:      uint(txData.Coin.Id),
			ValidatorId: vId,
			Value:       txData.Value,
		}

		err = s.repository.AddUnbond(unbond)
		if err != nil {
			s.logger.Error(err)
		}

		//adr, err := s.addressRepository.FindById(uint(tx.FromAddressID))
		//if err != nil {
		//	s.logger.Error(err)
		//} else {
		//	if err = s.UpdateWaitList(adr, txData.PubKey); err != nil {
		//		s.logger.Error(err)
		//	}
		//}

	}
}

//func (s *Service) UpdateWaitListWorker(data <-chan *models.Transaction) {
//	for tx := range data {
//
//		var adr, pk string
//
//		adr, err := s.addressRepository.FindById(uint(tx.FromAddressID))
//		if err != nil {
//			s.logger.Error(err)
//			continue
//		}
//
//		if transaction.Type(tx.Type) == transaction.TypeUnbond {
//			txData := new(api_pb.UnbondData)
//			if err = tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
//				s.logger.Error(err)
//				continue
//			}
//			pk = txData.PubKey
//		}
//		if transaction.Type(tx.Type) == transaction.TypeDelegate {
//			txData := new(api_pb.DelegateData)
//			if err = tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
//				s.logger.Error(err)
//				continue
//			}
//			pk = txData.PubKey
//		}
//
//		start := time.Now()
//		if err = s.UpdateWaitList(adr, pk); err != nil {
//			s.logger.Error(err)
//		}
//		elapsed := time.Since(start)
//		critical := 5 * time.Second
//		if elapsed > critical {
//			s.logger.Error(fmt.Sprintf("WaitList updating time: %s", elapsed))
//		} else {
//			s.logger.Info(fmt.Sprintf("WaitList updating time: %s", elapsed))
//		}
//	}
//}

func (s *Service) UpdateValidatorsWorker(jobs <-chan uint64) {
	for height := range jobs {
		status, err := s.nodeApi.Status()
		if err != nil {
			s.logger.Error(err)
			continue
		}
		if status.LatestBlockHeight-height > ChasingModDiff {
			continue
		}

		if height%UpdateTimoutInBlocks != 0 {
			continue
		}

		start := time.Now()
		resp, err := s.nodeApi.Candidates(false)
		if err != nil {
			s.logger.WithField("Block", height).Error(err)
		}
		elapsed := time.Since(start)
		s.logger.Info(fmt.Sprintf("Block: %d Candidate's data getting time: %s", height, elapsed))

		if len(resp.Candidates) <= 0 {
			continue
		}

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

		err = s.addressRepository.SaveFromMapIfNotExists(addressesMap)
		if err != nil {
			s.logger.Error(err)
		}

		for _, validator := range resp.Candidates {
			updateAt := time.Now()
			totalStake := validator.TotalStake
			status := uint8(validator.Status)
			commission := validator.Commission

			id, err := s.repository.FindIdByPkOrCreate(helpers.RemovePrefix(validator.PublicKey))
			if err != nil {
				s.logger.Error(err)
				continue
			}

			v, err := s.repository.GetById(id)
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
			controlAddressID, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(validator.OwnerAddress))
			if err != nil {
				s.logger.Error(err)
				continue
			}
			v.Status = &status
			v.TotalStake = &totalStake
			v.UpdateAt = &updateAt
			v.Commission = &commission
			v.OwnerAddressID = &ownerAddressID
			v.ControlAddressID = &controlAddressID
			v.RewardAddressID = &rewardAddressID
			validators = append(validators, v)
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

//func (s *Service) UpdateStakesWorker(blockHeight <-chan uint64) {
//	for height := range blockHeight {
//		start := time.Now()
//		status, err := s.nodeApi.Status()
//		if err != nil {
//			s.logger.Error(err)
//			continue
//		}
//
//		if status.LatestBlockHeight-height > ChasingModDiff {
//			continue
//		}
//
//		if height%UpdateTimoutInBlocks != 0 {
//			continue
//		}
//
//		s.logger.Warning("UPDATING STAKES")
//
//		resp, err := s.nodeApi.CandidatesExtended(true, false, "")
//		if err != nil {
//			s.logger.WithField("Block", height).Error(err)
//			continue
//		}
//
//		var (
//			stakes       []*models.Stake
//			validatorIds = make([]uint64, len(resp.Candidates))
//			addressesMap = make(map[string]struct{})
//		)
//
//		validatorsPkMap := make(map[string]struct{})
//		// Collect all PubKey's and addresses for save it before
//		for _, vlr := range resp.Candidates {
//			validatorsPkMap[helpers.RemovePrefix(vlr.PublicKey)] = struct{}{}
//			addressesMap[helpers.RemovePrefix(vlr.RewardAddress)] = struct{}{}
//			addressesMap[helpers.RemovePrefix(vlr.OwnerAddress)] = struct{}{}
//			for _, stake := range vlr.Stakes {
//				addressesMap[helpers.RemovePrefix(stake.Owner)] = struct{}{}
//			}
//		}
//
//		err = s.repository.SaveAllIfNotExist(validatorsPkMap)
//		if err != nil {
//			s.logger.Error(err)
//			continue
//		}
//
//		err = s.addressRepository.SaveFromMapIfNotExists(addressesMap)
//		if err != nil {
//			s.logger.Error(err)
//			continue
//		}
//
//		for i, vlr := range resp.Candidates {
//			id, err := s.repository.FindIdByPk(helpers.RemovePrefix(vlr.PublicKey))
//			if err != nil {
//				s.logger.WithField("pk", vlr.PublicKey).Error(err)
//				continue
//			}
//			validatorIds[i] = uint64(id)
//			for _, stake := range vlr.Stakes {
//				ownerAddressID, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(stake.Owner))
//				if err != nil {
//					s.logger.Error(err)
//					continue
//				}
//				stakes = append(stakes, &models.Stake{
//					ValidatorID:    id,
//					OwnerAddressID: ownerAddressID,
//					CoinID:         uint(stake.Coin.Id),
//					Value:          stake.Value,
//					BipValue:       stake.BipValue,
//				})
//			}
//		}
//
//		chunksCount := int(math.Ceil(float64(len(stakes)) / float64(s.env.StakeChunkSize)))
//
//		wg := new(sync.WaitGroup)
//		wg.Add(chunksCount)
//
//		for i := 0; i < chunksCount; i++ {
//			start := s.env.StakeChunkSize * i
//			end := start + s.env.StakeChunkSize
//			if end > len(stakes) {
//				end = len(stakes)
//			}
//
//			go func(stakes []*models.Stake) {
//				err = s.repository.SaveAllStakes(stakes)
//				if err != nil {
//					var coinsList []string
//					coinMap := make(map[uint]struct{})
//					for _, s := range stakes {
//						coinMap[s.CoinID] = struct{}{}
//					}
//					for id := range coinMap {
//						coinsList = append(coinsList, fmt.Sprintf("%d", id))
//					}
//					s.logger.WithFields(logrus.Fields{
//						"coins": strings.Join(coinsList, ","),
//						"block": height,
//					}).Fatal(err)
//				}
//				wg.Done()
//			}(stakes[start:end])
//		}
//
//		wg.Wait()
//		stakesId := make([]uint64, len(stakes))
//		for i, stake := range stakes {
//			stakesId[i] = uint64(stake.ID)
//			//err = s.UpdateWaitListByStake(stake)
//			//if err != nil {
//			//	s.logger.Error(err)
//			//}
//		}
//
//		err = s.repository.DeleteStakesNotInListIds(stakesId)
//		if err != nil {
//			s.logger.Error(err)
//		}
//
//		elapsed := time.Since(start)
//		s.logger.Warning(fmt.Sprintf("Stake has been updated. Block: %d Processing time: %s", height, elapsed))
//	}
//}

// HandleBlockResponse Get validators PK from response and store it to validators table if not exist
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
		if transaction.Type(tx.Type) == transaction.TypeDeclareCandidacy {
			txData := new(api_pb.DeclareCandidacyData)
			if err := tx.Data.UnmarshalTo(txData); err != nil {
				return err
			}

			_, err := s.repository.FindIdByPkOrCreate(helpers.RemovePrefix(txData.PubKey))
			if err != nil {
				return err
			}
		}

		if transaction.Type(tx.Type) == transaction.TypeEditCandidatePublicKey {
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
	status := uint8(response.Status)
	validator.Status = &status
	validator.TotalStake = &response.TotalStake
	commission := response.Commission
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
		stakes = append(stakes, &models.Stake{
			CoinID:         uint(stake.Coin.Id),
			Value:          stake.Value,
			ValidatorID:    validatorID,
			BipValue:       stake.BipValue,
			OwnerAddressID: ownerAddressID,
		})
	}
	return stakes, nil
}

//func (s *Service) UpdateWaitList(adr, pk string) error {
//	var err error
//	var addressId uint
//	var data *api_pb.WaitListResponse
//
//	strRune := []rune(adr)
//	prefix := string(strRune[0:2])
//
//	if strings.ToLower(prefix) == "mx" {
//		data, err = s.nodeApi.WaitList(pk, adr)
//	} else {
//		data, err = s.nodeApi.WaitList(pk, fmt.Sprintf("Mx%s", adr))
//	}
//	if err != nil {
//		return err
//	}
//
//	if strings.ToLower(prefix) == "mx" {
//		addressId, err = s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(adr))
//	} else {
//		addressId, err = s.addressRepository.FindIdOrCreate(adr)
//	}
//	if err != nil {
//		return err
//	}
//
//	vId, err := s.repository.FindIdByPk(helpers.RemovePrefix(pk))
//	if err != nil {
//		return err
//	}
//
//	if len(data.List) == 0 {
//		return s.repository.RemoveFromWaitList(addressId, vId)
//	}
//
//	var existCoins []uint64
//	var stakes []*models.Stake
//
//	for _, item := range data.List {
//		existCoins = append(existCoins, item.Coin.Id)
//
//		bipValue := "0"
//		if item.Coin.Id == 0 {
//			bipValue = item.Value
//		}
//
//		stakes = append(stakes, &models.Stake{
//			OwnerAddressID: addressId,
//			CoinID:         uint(item.Coin.Id),
//			ValidatorID:    vId,
//			Value:          item.Value,
//			BipValue:       bipValue,
//			IsKicked:       true,
//		})
//
//	}
//
//	err = s.repository.UpdateStakes(stakes)
//	if err != nil {
//		s.logger.Error(err)
//	}
//
//	return s.repository.DeleteFromWaitList(addressId, vId, existCoins)
//}

//func (s *Service) UpdateWaitListByStake(stake *models.Stake) error {
//	var data *api_pb.WaitListResponse
//
//	adr, err := s.addressRepository.FindById(stake.OwnerAddressID)
//	if err != nil {
//		return err
//	}
//
//	pk, err := s.repository.GetById(stake.ValidatorID)
//	if err != nil {
//		return err
//	}
//
//	data, err = s.nodeApi.WaitList(pk.GetPublicKey(), fmt.Sprintf("Mx%s", adr))
//	if err != nil {
//		return err
//	}
//
//	var existCoins []uint64
//	var stakes []*models.Stake
//
//	for _, item := range data.List {
//		existCoins = append(existCoins, item.Coin.Id)
//
//		bipValue := "0"
//		if item.Coin.Id == 0 {
//			bipValue = item.Value
//		}
//
//		stakes = append(stakes, &models.Stake{
//			OwnerAddressID: stake.OwnerAddressID,
//			CoinID:         uint(item.Coin.Id),
//			ValidatorID:    stake.ValidatorID,
//			Value:          item.Value,
//			BipValue:       bipValue,
//			IsKicked:       true,
//		})
//	}
//
//	if len(stakes) > 0 {
//		err = s.repository.UpdateStakes(stakes)
//		if err != nil {
//			s.logger.Error(err)
//		}
//		return s.repository.DeleteFromWaitList(stake.OwnerAddressID, stake.ValidatorID, existCoins)
//	}
//	return nil
//}

func (s *Service) GetUnbondBlockCount() uint64 {
	if s.env.BaseCoin == "MNT" {
		return UnbondBlockCountTestnet
	}
	return UnbondBlockCount
}

func (s *Service) GetMoveStakeBlockCount() uint64 {
	if s.env.BaseCoin == "MNT" {
		return MoveStakeBlockCountTestnet
	}
	return MoveStakeBlockCount
}
