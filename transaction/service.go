package transaction

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/MinterTeam/minter-explorer-extender/v2/address"
	"github.com/MinterTeam/minter-explorer-extender/v2/broadcast"
	"github.com/MinterTeam/minter-explorer-extender/v2/coin"
	"github.com/MinterTeam/minter-explorer-extender/v2/env"
	"github.com/MinterTeam/minter-explorer-extender/v2/liquidity_pool"
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/MinterTeam/minter-explorer-extender/v2/validator"
	"github.com/MinterTeam/minter-explorer-tools/v4/helpers"
	"github.com/MinterTeam/minter-go-sdk/v2/transaction"
	"github.com/MinterTeam/node-grpc-gateway/api_pb"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"
	"math"
	"strconv"
	"time"
)

type Service struct {
	env                 *env.ExtenderEnvironment
	txRepository        *Repository
	addressRepository   *address.Repository
	validatorRepository *validator.Repository
	coinRepository      *coin.Repository
	coinService         *coin.Service
	lpRepository        *liquidity_pool.Repository
	broadcastService    *broadcast.Service
	jobSaveTxs          chan []*models.Transaction
	jobSaveTxsOutput    chan []*models.Transaction
	jobSaveValidatorTxs chan []*models.TransactionValidator
	jobSaveInvalidTxs   chan []*models.InvalidTransaction
	jobUnbondSaver      chan *models.Transaction
	jobMoveStake        chan *api_pb.TransactionResponse
	logger              *logrus.Entry
}

func NewService(env *env.ExtenderEnvironment, repository *Repository, addressRepository *address.Repository,
	validatorRepository *validator.Repository, coinRepository *coin.Repository, coinService *coin.Service,
	broadcastService *broadcast.Service, logger *logrus.Entry,
	jobUnbondSaver chan *models.Transaction, liquidityPoolRepository *liquidity_pool.Repository,
	jobMoveStake chan *api_pb.TransactionResponse) *Service {

	return &Service{
		env:                 env,
		txRepository:        repository,
		coinRepository:      coinRepository,
		addressRepository:   addressRepository,
		coinService:         coinService,
		validatorRepository: validatorRepository,
		lpRepository:        liquidityPoolRepository,
		broadcastService:    broadcastService,
		jobSaveTxs:          make(chan []*models.Transaction, env.WrkSaveTxsCount),
		jobSaveTxsOutput:    make(chan []*models.Transaction, env.WrkSaveTxsOutputCount),
		jobSaveValidatorTxs: make(chan []*models.TransactionValidator, env.WrkSaveValidatorTxsCount),
		jobSaveInvalidTxs:   make(chan []*models.InvalidTransaction, env.WrkSaveInvTxsCount),
		jobUnbondSaver:      jobUnbondSaver,
		jobMoveStake:        jobMoveStake,
		logger:              logger,
	}
}

func (s *Service) GetSaveTxJobChannel() chan []*models.Transaction {
	return s.jobSaveTxs
}
func (s *Service) GetSaveTxsOutputJobChannel() chan []*models.Transaction {
	return s.jobSaveTxsOutput
}
func (s *Service) GetSaveInvalidTxsJobChannel() chan []*models.InvalidTransaction {
	return s.jobSaveInvalidTxs
}
func (s *Service) GetSaveTxValidatorJobChannel() chan []*models.TransactionValidator {
	return s.jobSaveValidatorTxs
}

// HandleTransactionsFromBlockResponse Handle response and save block to DB
func (s *Service) HandleTransactionsFromBlockResponse(blockHeight uint64, blockCreatedAt time.Time,
	transactions []*api_pb.TransactionResponse) error {

	var txList []*models.Transaction
	var invalidTxList []*models.InvalidTransaction

	for _, tx := range transactions {
		if tx.Log == "" {
			txn, err := s.handleValidTransaction(tx, blockHeight, blockCreatedAt)
			if err != nil {
				s.logger.Error(err)
				return err
			}
			txList = append(txList, txn)
			switch transaction.Type(tx.Type) {
			//case transaction.TypeCreateSwapPool:
			//	err := s.liquidityPoolService.CreateLiquidityPool(tx)
			//	if err != nil {
			//		return err
			//	}
			case transaction.TypeDelegate,
				transaction.TypeUnbond:
				s.broadcastService.StakeChannel() <- tx
			case transaction.TypeMoveStake:
				s.jobMoveStake <- tx
			}
		} else {
			txn, err := s.handleInvalidTransaction(tx, blockHeight, blockCreatedAt)
			if err != nil {
				s.logger.Error(err)
				return err
			}
			invalidTxList = append(invalidTxList, txn)
		}
	}

	if len(txList) > 0 {
		s.GetSaveTxJobChannel() <- txList
		s.coinService.GetUpdateCoinsFromTxsJobChannel() <- txList
	}

	if len(invalidTxList) > 0 {
		s.GetSaveInvalidTxsJobChannel() <- invalidTxList
	}

	return nil
}

func (s *Service) SaveTransactionsWorker(jobs <-chan []*models.Transaction) {
	for transactions := range jobs {
		err := s.txRepository.SaveAll(transactions)
		if err != nil {
			s.logger.Fatal(err)
		}

		links, err := s.getLinksTxValidator(transactions)
		if err != nil {
			s.logger.Fatal(err)
		}
		if len(links) > 0 {
			chunksCount := int(math.Ceil(float64(len(links)) / float64(s.env.TxChunkSize)))
			for i := 0; i < chunksCount; i++ {
				start := s.env.TxChunkSize * i
				end := start + s.env.TxChunkSize
				if end > len(links) {
					end = len(links)
				}
				s.GetSaveTxValidatorJobChannel() <- links[start:end]
			}
		}

		lpLinks, err := s.getLinksLiquidityPool(transactions)
		if err != nil {
			s.logger.Error(err)
		}
		if len(lpLinks) > 0 {
			err = s.txRepository.LinkWithLiquidityPool(lpLinks)
		}
		if err != nil {
			s.logger.Error(err)
		}
		//s.liquidityPoolService.LiquidityPoolTradesChannel() <- transactions
		s.GetSaveTxsOutputJobChannel() <- transactions
		s.broadcastService.TransactionsChannel() <- transactions
	}
}
func (s *Service) SaveTransactionsOutputWorker(jobs <-chan []*models.Transaction) {
	for transactions := range jobs {
		err := s.SaveAllTxOutputs(transactions)
		if err != nil {
			s.logger.Fatal(err)
		}
	}
}
func (s *Service) SaveInvalidTransactionsWorker(jobs <-chan []*models.InvalidTransaction) {
	for transactions := range jobs {
		err := s.txRepository.SaveAllInvalid(transactions)
		if err != nil {
			s.logger.Error(err)
		}
		helpers.HandleError(err)
	}
}

func (s *Service) SaveTxValidatorWorker(jobs <-chan []*models.TransactionValidator) {
	for links := range jobs {
		err := s.txRepository.LinkWithValidators(links)
		if err != nil {
			s.logger.Error(err)
		}
		helpers.HandleError(err)
	}
}

func (s *Service) UpdateTxsIndexWorker() {
	for {
		err := s.txRepository.IndexLastNTxAddress(s.env.WrkUpdateTxsIndexNumBlocks)
		if err != nil {
			s.logger.Error(err)
		}
		time.Sleep(time.Duration(s.env.WrkUpdateTxsIndexTime) * time.Second)
	}
}

func (s *Service) SaveAllTxOutputs(txList []*models.Transaction) error {
	var (
		list      []*models.TransactionOutput
		checkList []*models.Check
		idsList   []uint64
	)

	for _, tx := range txList {
		if tx.ID == 0 {
			return errors.New("no transaction id")
		}

		idsList = append(idsList, tx.ID)

		if transaction.Type(tx.Type) != transaction.TypeSend &&
			transaction.Type(tx.Type) != transaction.TypeMultisend &&
			transaction.Type(tx.Type) != transaction.TypeRedeemCheck &&
			transaction.Type(tx.Type) != transaction.TypeEditCoinOwner &&
			transaction.Type(tx.Type) != transaction.TypeEditCandidate &&
			transaction.Type(tx.Type) != transaction.TypeUnbond &&
			transaction.Type(tx.Type) != transaction.TypeDelegate {
			continue
		}

		if transaction.Type(tx.Type) == transaction.TypeSend {
			txData := new(api_pb.SendData)
			if err := tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
				return err
			}
			toId, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(txData.To))
			if err != nil {
				return err
			}
			list = append(list, &models.TransactionOutput{
				TransactionID: tx.ID,
				ToAddressID:   uint64(toId),
				CoinID:        uint(txData.Coin.Id),
				Value:         txData.Value,
			})
		}
		if transaction.Type(tx.Type) == transaction.TypeMultisend {
			txData := new(api_pb.MultiSendData)
			if err := tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
				return err
			}
			for _, receiver := range txData.List {
				toId, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(receiver.To))
				if err != nil {
					return err
				}
				list = append(list, &models.TransactionOutput{
					TransactionID: tx.ID,
					ToAddressID:   uint64(toId),
					CoinID:        uint(receiver.Coin.Id),
					Value:         receiver.Value,
				})
			}
		}

		if transaction.Type(tx.Type) == transaction.TypeRedeemCheck {
			txData := new(api_pb.RedeemCheckData)
			if err := tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
				return err
			}
			data, err := transaction.DecodeCheckBase64(txData.RawCheck)
			if err != nil {
				s.logger.WithFields(logrus.Fields{
					"Tx": tx.Hash,
				}).Error(err)
				return err
			}
			sender, err := data.Sender()
			if err != nil {
				s.logger.Error(err)
				return err
			}
			// We are put a creator of a check into "to" field
			// because "from" field use for a person who created a transaction
			toId, err := s.addressRepository.FindId(helpers.RemovePrefix(sender))
			if err != nil {
				s.logger.Fatal(err)
				return err
			}

			rawCheck, err := base64.StdEncoding.DecodeString(txData.RawCheck)
			if err != nil {
				s.logger.Fatal(err)
				return err
			}

			checkList = append(checkList, &models.Check{
				TransactionID: tx.ID,
				Data:          hex.EncodeToString(rawCheck),
				ToAddressId:   uint(tx.FromAddressID),
				FromAddressId: toId,
			})

			list = append(list, &models.TransactionOutput{
				TransactionID: tx.ID,
				ToAddressID:   uint64(toId),
				CoinID:        uint(data.Coin),
				Value:         data.Value.String(),
			})
		}

		if transaction.Type(tx.Type) == transaction.TypeEditCoinOwner {
			txData := new(api_pb.EditCoinOwnerData)
			if err := tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
				return err
			}

			err := s.coinService.ChangeOwner(txData.Symbol, txData.NewOwner)
			if err != nil {
				return err
			}
		}

		if transaction.Type(tx.Type) == transaction.TypeEditCandidate {
			txData := new(api_pb.EditCandidateData)
			if err := tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
				return err
			}

			vId, err := s.validatorRepository.FindIdByPk(helpers.RemovePrefix(txData.PubKey))
			if err != nil {
				return err
			}

			v, err := s.validatorRepository.GetById(vId)
			if err != nil {
				return err
			}

			newOwnerAddressId, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(txData.OwnerAddress))
			if err != nil {
				return err
			}
			newControlAddressId, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(txData.ControlAddress))
			if err != nil {
				return err
			}
			newRewardAddressId, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(txData.RewardAddress))
			if err != nil {
				return err
			}

			v.OwnerAddressID = &newOwnerAddressId
			v.ControlAddressID = &newControlAddressId
			v.RewardAddressID = &newRewardAddressId

			err = s.validatorRepository.Update(v)
			if err != nil {
				return err
			}
		}

		if transaction.Type(tx.Type) == transaction.TypeUnbond {
			s.jobUnbondSaver <- tx
		}
	}

	if len(list) > 0 {
		err := s.txRepository.SaveAllTxOutputs(list)
		if err != nil {
			return err
		}
	}
	if len(idsList) > 0 {
		err := s.txRepository.IndexTxAddress(idsList)
		if err != nil {
			return err
		}
	}
	if len(checkList) > 0 {
		err := s.txRepository.SaveRedeemedChecks(checkList)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) handleValidTransaction(tx *api_pb.TransactionResponse, blockHeight uint64, blockCreatedAt time.Time) (*models.Transaction, error) {
	fromId, err := s.addressRepository.FindId(helpers.RemovePrefix(tx.From))
	if err != nil {
		return nil, err
	}

	rawTxData := make([]byte, hex.DecodedLen(len(tx.RawTx)))
	rawTx, err := hex.Decode(rawTxData, []byte(tx.RawTx))
	if err != nil {
		return nil, err
	}

	txTags := tx.GetTags()

	txDataJson, err := txDataJson(tx.Type, tx.Data)
	if err != nil {
		return nil, err
	}

	return &models.Transaction{
		FromAddressID: uint64(fromId),
		BlockID:       blockHeight,
		Nonce:         tx.Nonce,
		GasPrice:      tx.GasPrice,
		Gas:           tx.Gas,
		GasCoinID:     tx.GasCoin.Id,
		Commission:    txTags["tx.commission_in_base_coin"],
		CreatedAt:     blockCreatedAt,
		Type:          uint8(tx.Type),
		Hash:          helpers.RemovePrefix(tx.Hash),
		ServiceData:   string(tx.ServiceData),
		IData:         tx.Data,
		Data:          txDataJson,
		Tags:          txTags,
		Payload:       tx.Payload,
		RawTx:         rawTxData[:rawTx],
	}, nil
}

func (s *Service) handleInvalidTransaction(tx *api_pb.TransactionResponse, blockHeight uint64, blockCreatedAt time.Time) (*models.InvalidTransaction, error) {
	fromId, err := s.addressRepository.FindId(helpers.RemovePrefix(tx.From))
	if err != nil {
		return nil, err
	}

	txDataJson, err := txDataJson(tx.Type, tx.Data)
	if err != nil {
		return nil, err
	}

	txTags := tx.GetTags()

	rawTxData := make([]byte, hex.DecodedLen(len(tx.RawTx)))
	rawTx, err := hex.Decode(rawTxData, []byte(tx.RawTx))
	if err != nil {
		return nil, err
	}

	return &models.InvalidTransaction{
		FromAddressID: uint64(fromId),
		BlockID:       blockHeight,
		CreatedAt:     blockCreatedAt,
		Type:          uint8(tx.Type),
		Hash:          helpers.RemovePrefix(tx.Hash),
		TxData:        string(txDataJson),
		Log:           tx.Log,
		Nonce:         tx.Nonce,
		GasPrice:      tx.GasPrice,
		Gas:           tx.Gas,
		Commission:    txTags["tx.commission_in_base_coin"],
		GasCoinID:     tx.GasCoin.Id,
		ServiceData:   string(tx.ServiceData),
		Tags:          txTags,
		Payload:       tx.Payload,
		RawTx:         rawTxData[:rawTx],
	}, nil
}

func (s *Service) getLinksTxValidator(transactions []*models.Transaction) ([]*models.TransactionValidator, error) {
	var links []*models.TransactionValidator

	for _, tx := range transactions {
		if tx.ID == 0 {
			return nil, errors.New("no transaction id")
		}
		var validatorPk string
		switch transaction.Type(tx.Type) {
		case transaction.TypeDeclareCandidacy:
			txData := new(api_pb.DeclareCandidacyData)
			if err := tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
				return nil, err
			}
			validatorPk = txData.PubKey
		case transaction.TypeDelegate:
			txData := new(api_pb.DelegateData)
			if err := tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
				return nil, err
			}
			validatorPk = txData.PubKey
		case transaction.TypeUnbond:
			txData := new(api_pb.UnbondData)
			if err := tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
				return nil, err
			}
			validatorPk = txData.PubKey
		case transaction.TypeSetCandidateOnline:
			txData := new(api_pb.SetCandidateOnData)
			if err := tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
				return nil, err
			}
			validatorPk = txData.PubKey
		case transaction.TypeSetCandidateOffline:
			txData := new(api_pb.SetCandidateOffData)
			if err := tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
				return nil, err
			}
			validatorPk = txData.PubKey
		case transaction.TypeEditCandidate:
			txData := new(api_pb.EditCandidateData)
			if err := tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
				return nil, err
			}
			validatorPk = txData.PubKey
		case transaction.TypeEditCandidatePublicKey:
			txData := new(api_pb.EditCandidatePublicKeyData)
			if err := tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
				return nil, err
			}
			validatorPk = txData.PubKey
		case transaction.TypeMoveStake:
			txData := new(api_pb.MoveStakeData)
			if err := tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
				return nil, err
			}
			validatorPk = txData.FromPubKey
			validatorId, err := s.validatorRepository.FindIdByPkOrCreate(helpers.RemovePrefix(txData.ToPubKey))
			if err == nil {
				links = append(links, &models.TransactionValidator{
					TransactionID: tx.ID,
					ValidatorID:   uint64(validatorId),
				})
			}
		}

		if validatorPk != "" {
			validatorId, err := s.validatorRepository.FindIdByPkOrCreate(helpers.RemovePrefix(validatorPk))
			if err != nil {
				return nil, err
			}
			links = append(links, &models.TransactionValidator{
				TransactionID: tx.ID,
				ValidatorID:   uint64(validatorId),
			})
		}
	}

	return links, nil
}

func (s *Service) getLinksLiquidityPool(transactions []*models.Transaction) ([]*models.TransactionLiquidityPool, error) {
	var links []*models.TransactionLiquidityPool
	for _, tx := range transactions {
		switch transaction.Type(tx.Type) {
		case transaction.TypeSellSwapPool,
			transaction.TypeBuySwapPool,
			transaction.TypeSellAllSwapPool:
			lpList, err := s.lpRepository.GetPoolsByTxTags(tx.Tags)
			if err != nil {
				return nil, err
			}
			for _, lp := range lpList {
				links = append(links, &models.TransactionLiquidityPool{
					TransactionID:   tx.ID,
					LiquidityPoolID: lp.Id,
				})
			}
		case transaction.TypeCreateSwapPool:
			lpId, err := strconv.ParseUint(tx.Tags["tx.pool_id"], 10, 64)
			if err != nil {
				return nil, err
			}
			links = append(links, &models.TransactionLiquidityPool{
				TransactionID:   tx.ID,
				LiquidityPoolID: lpId,
			})
		case transaction.TypeRemoveLiquidity,
			transaction.TypeAddLiquidity:
			lp, err := s.lpRepository.GetPoolByPairString(tx.Tags["tx.pair_ids"])
			if err != nil {
				return nil, err
			}
			links = append(links, &models.TransactionLiquidityPool{
				TransactionID:   tx.ID,
				LiquidityPoolID: lp.Id,
			})
		}
	}
	return links, nil
}

func txDataJson(txType uint64, data *any.Any) ([]byte, error) {
	mo := protojson.MarshalOptions{EmitUnpopulated: true}
	switch transaction.Type(txType) {
	case transaction.TypeSend:
		txData := new(api_pb.SendData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeSellCoin:
		txData := new(api_pb.SellCoinData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeSellAllCoin:
		txData := new(api_pb.SellAllCoinData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeBuyCoin:
		txData := new(api_pb.BuyCoinData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeCreateCoin:
		txData := new(api_pb.CreateCoinData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeDeclareCandidacy:
		txData := new(api_pb.DeclareCandidacyData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeDelegate:
		txData := new(api_pb.DelegateData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeUnbond:
		txData := new(api_pb.UnbondData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeRedeemCheck:
		txData := new(api_pb.RedeemCheckData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeSetCandidateOnline:
		txData := new(api_pb.SetCandidateOnData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeSetCandidateOffline:
		txData := new(api_pb.SetCandidateOffData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeCreateMultisig:
		txData := new(api_pb.CreateMultisigData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeMultisend:
		txData := new(api_pb.MultiSendData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeEditCandidate:
		txData := new(api_pb.EditCandidateData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeSetHaltBlock:
		txData := new(api_pb.SetHaltBlockData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeRecreateCoin:
		txData := new(api_pb.RecreateCoinData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeEditCoinOwner:
		txData := new(api_pb.EditCoinOwnerData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeEditMultisig:
		txData := new(api_pb.EditMultisigData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeEditCandidatePublicKey:
		txData := new(api_pb.EditCandidatePublicKeyData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeAddLiquidity:
		txData := new(api_pb.AddLiquidityData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeRemoveLiquidity:
		txData := new(api_pb.RemoveLiquidityData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeSellSwapPool:
		txData := new(api_pb.SellSwapPoolData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeBuySwapPool:
		txData := new(api_pb.BuySwapPoolData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeSellAllSwapPool:
		txData := new(api_pb.SellAllSwapPoolData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeEditCommissionCandidate:
		txData := new(api_pb.EditCandidateCommission)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeMintToken:
		txData := new(api_pb.MintTokenData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeBurnToken:
		txData := new(api_pb.BurnTokenData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeCreateToken:
		txData := new(api_pb.CreateTokenData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeRecreateToken:
		txData := new(api_pb.RecreateTokenData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeCreateSwapPool:
		txData := new(api_pb.CreateSwapPoolData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeVoteCommission:
		txData := new(api_pb.VoteCommissionData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeVoteUpdate:
		txData := new(api_pb.VoteUpdateData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeAddLimitOrder:
		txData := new(api_pb.AddLimitOrderData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeRemoveLimitOrder:
		txData := new(api_pb.RemoveLimitOrderData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeLockStake:
		txData := new(api_pb.LockStakeData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeLock:
		txData := new(api_pb.LockData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeMoveStake:
		txData := new(api_pb.MoveStakeData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := mo.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	}

	return nil, errors.New(fmt.Sprintf("unknown tx type: %d", txType))
}
