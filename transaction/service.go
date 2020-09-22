package transaction

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/MinterTeam/minter-explorer-extender/v2/address"
	"github.com/MinterTeam/minter-explorer-extender/v2/broadcast"
	"github.com/MinterTeam/minter-explorer-extender/v2/coin"
	"github.com/MinterTeam/minter-explorer-extender/v2/env"
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/MinterTeam/minter-explorer-extender/v2/validator"
	"github.com/MinterTeam/minter-explorer-tools/v4/helpers"
	"github.com/MinterTeam/minter-go-sdk/v2/transaction"
	"github.com/MinterTeam/node-grpc-gateway/api_pb"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/sirupsen/logrus"
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
	broadcastService    *broadcast.Service
	jobSaveTxs          chan []*models.Transaction
	jobSaveTxsOutput    chan []*models.Transaction
	jobSaveValidatorTxs chan []*models.TransactionValidator
	jobSaveInvalidTxs   chan []*models.InvalidTransaction
	jobUpdateWaitList   chan *models.Transaction
	jobUnbondSaver      chan *models.Transaction
	logger              *logrus.Entry
}

func NewService(env *env.ExtenderEnvironment, repository *Repository, addressRepository *address.Repository,
	validatorRepository *validator.Repository, coinRepository *coin.Repository, coinService *coin.Service,
	broadcastService *broadcast.Service, logger *logrus.Entry, jobUpdateWaitList chan *models.Transaction,
	jobUnbondSaver chan *models.Transaction) *Service {
	return &Service{
		env:                 env,
		txRepository:        repository,
		coinRepository:      coinRepository,
		addressRepository:   addressRepository,
		coinService:         coinService,
		validatorRepository: validatorRepository,
		broadcastService:    broadcastService,
		jobSaveTxs:          make(chan []*models.Transaction, env.WrkSaveTxsCount),
		jobSaveTxsOutput:    make(chan []*models.Transaction, env.WrkSaveTxsOutputCount),
		jobSaveValidatorTxs: make(chan []*models.TransactionValidator, env.WrkSaveValidatorTxsCount),
		jobSaveInvalidTxs:   make(chan []*models.InvalidTransaction, env.WrkSaveInvTxsCount),
		jobUpdateWaitList:   jobUpdateWaitList,
		jobUnbondSaver:      jobUnbondSaver,
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

//Handle response and save block to DB
func (s *Service) HandleTransactionsFromBlockResponse(blockHeight uint64, blockCreatedAt time.Time, transactions []*api_pb.BlockResponse_Transaction) error {

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

		s.GetSaveTxsOutputJobChannel() <- transactions
		go s.broadcastService.PublishTransactions(transactions)
	}
}
func (s *Service) SaveTransactionsOutputWorker(jobs <-chan []*models.Transaction) {
	for transactions := range jobs {
		err := s.SaveAllTxOutputs(transactions)
		if err != nil {
			s.logger.Error(err)
		}
		helpers.HandleError(err)
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
		list    []*models.TransactionOutput
		idsList []uint64
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
			transaction.Type(tx.Type) != transaction.TypeEditCandidatePublicKey &&
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
			helpers.HandleError(err)
			coinID, err := strconv.ParseUint(txData.Coin.Id, 10, 64)
			helpers.HandleError(err)
			list = append(list, &models.TransactionOutput{
				TransactionID: tx.ID,
				ToAddressID:   uint64(toId),
				CoinID:        uint(coinID),
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
				helpers.HandleError(err)
				coinID, err := strconv.ParseUint(receiver.Coin.Id, 10, 64)
				helpers.HandleError(err)
				list = append(list, &models.TransactionOutput{
					TransactionID: tx.ID,
					ToAddressID:   uint64(toId),
					CoinID:        uint(coinID),
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
			}

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

		if transaction.Type(tx.Type) == transaction.TypeEditCandidatePublicKey {
			txData := new(api_pb.EditCandidatePublicKeyData)
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

			err = s.validatorRepository.AddPk(vId, helpers.RemovePrefix(txData.NewPubKey))
			if err != nil {
				return err
			}

			v.PublicKey = helpers.RemovePrefix(txData.NewPubKey)
			err = s.validatorRepository.Update(v)
			if err != nil {
				return err
			}
		}

		if transaction.Type(tx.Type) == transaction.TypeUnbond {
			s.jobUnbondSaver <- tx
			s.jobUpdateWaitList <- tx
		}
		if transaction.Type(tx.Type) == transaction.TypeDelegate {
			s.jobUpdateWaitList <- tx
		}
	}

	if len(list) > 0 {
		err := s.txRepository.SaveAllTxOutputs(list)
		helpers.HandleError(err)
	}
	if len(idsList) > 0 {
		err := s.txRepository.IndexTxAddress(idsList)
		helpers.HandleError(err)
	}

	return nil
}

func (s *Service) handleValidTransaction(tx *api_pb.BlockResponse_Transaction, blockHeight uint64, blockCreatedAt time.Time) (*models.Transaction, error) {
	fromId, err := s.addressRepository.FindId(helpers.RemovePrefix(tx.From))
	if err != nil {
		return nil, err
	}
	nonce, err := strconv.ParseUint(tx.Nonce, 10, 64)
	if err != nil {
		return nil, err
	}
	gas, err := strconv.ParseUint(tx.Gas, 10, 64)
	if err != nil {
		return nil, err
	}
	gasCoin, err := strconv.ParseUint(tx.GasCoin, 10, 64)
	if err != nil {
		return nil, err
	}
	rawTxData := make([]byte, hex.DecodedLen(len(tx.RawTx)))
	rawTx, err := hex.Decode(rawTxData, []byte(tx.RawTx))
	if err != nil {
		return nil, err
	}
	txTagsJson, err := json.Marshal(tx.Tags)
	if err != nil {
		return nil, err
	}
	txTags := make(map[string]string)
	err = json.Unmarshal(txTagsJson, &txTags)
	if err != nil {
		return nil, err
	}

	txGasPrice, err := strconv.ParseUint(tx.GasPrice, 10, 64)
	if err != nil {
		return nil, err
	}

	txType, err := strconv.ParseUint(tx.Type, 10, 64)
	if err != nil {
		return nil, err
	}

	txDataJson, err := txDataJson(txType, tx.Data)
	if err != nil {
		return nil, err
	}

	return &models.Transaction{
		FromAddressID: uint64(fromId),
		BlockID:       blockHeight,
		Nonce:         nonce,
		GasPrice:      txGasPrice,
		Gas:           gas,
		GasCoinID:     gasCoin,
		CreatedAt:     blockCreatedAt,
		Type:          uint8(txType),
		Hash:          helpers.RemovePrefix(tx.Hash),
		ServiceData:   string(tx.ServiceData),
		IData:         tx.Data,
		Data:          txDataJson,
		Tags:          txTags,
		Payload:       tx.Payload,
		RawTx:         rawTxData[:rawTx],
	}, nil
}

func (s *Service) handleInvalidTransaction(tx *api_pb.BlockResponse_Transaction, blockHeight uint64, blockCreatedAt time.Time) (*models.InvalidTransaction, error) {
	fromId, err := s.addressRepository.FindId(helpers.RemovePrefix(tx.From))
	if err != nil {
		return nil, err
	}
	txType, err := strconv.ParseUint(tx.Type, 10, 64)
	if err != nil {
		return nil, err
	}

	txDataJson, err := txDataJson(txType, tx.Data)
	if err != nil {
		return nil, err
	}

	return &models.InvalidTransaction{
		FromAddressID: uint64(fromId),
		BlockID:       blockHeight,
		CreatedAt:     blockCreatedAt,
		Type:          uint8(txType),
		Hash:          helpers.RemovePrefix(tx.Hash),
		TxData:        string(txDataJson),
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

func txDataJson(txType uint64, data *any.Any) ([]byte, error) {
	switch transaction.Type(txType) {
	case transaction.TypeSend:
		txData := new(api_pb.SendData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := json.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeSellCoin:
		txData := new(api_pb.SellCoinData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := json.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeSellAllCoin:
		txData := new(api_pb.SellAllCoinData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := json.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeBuyCoin:
		txData := new(api_pb.BuyCoinData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := json.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeCreateCoin:
		txData := new(api_pb.CreateCoinData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := json.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeDeclareCandidacy:
		txData := new(api_pb.DeclareCandidacyData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := json.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeDelegate:
		txData := new(api_pb.DelegateData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := json.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeUnbond:
		txData := new(api_pb.UnbondData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := json.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeRedeemCheck:
		txData := new(api_pb.RedeemCheckData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := json.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeSetCandidateOnline:
		txData := new(api_pb.SetCandidateOnData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := json.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeSetCandidateOffline:
		txData := new(api_pb.SetCandidateOffData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := json.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeCreateMultisig:
		txData := new(api_pb.CreateMultisigData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := json.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeMultisend:
		txData := new(api_pb.MultiSendData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := json.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeEditCandidate:
		txData := new(api_pb.EditCandidateData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := json.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeSetHaltBlock:
		txData := new(api_pb.SetHaltBlockData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := json.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeRecreateCoin:
		txData := new(api_pb.RecreateCoinData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := json.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeEditCoinOwner:
		txData := new(api_pb.EditCoinOwnerData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := json.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeEditMultisig:
		txData := new(api_pb.EditMultisigData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := json.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypePriceVote:
		txData := new(api_pb.PriceVoteData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := json.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	case transaction.TypeEditCandidatePublicKey:
		txData := new(api_pb.EditCandidatePublicKeyData)
		if err := data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		txDataJson, err := json.Marshal(txData)
		if err != nil {
			return nil, err
		}
		return txDataJson, nil
	}

	return nil, errors.New("unknown tx type")
}
