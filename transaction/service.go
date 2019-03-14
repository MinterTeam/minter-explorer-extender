package transaction

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/MinterTeam/minter-explorer-extender/address"
	"github.com/MinterTeam/minter-explorer-extender/broadcast"
	"github.com/MinterTeam/minter-explorer-extender/coin"
	"github.com/MinterTeam/minter-explorer-extender/validator"
	"github.com/MinterTeam/minter-explorer-tools/helpers"
	"github.com/MinterTeam/minter-explorer-tools/models"
	"github.com/daniildulin/minter-node-api/responses"
	"math"
	"strconv"
	"time"
)

type Service struct {
	env                 *models.ExtenderEnvironment
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
}

func NewService(env *models.ExtenderEnvironment, repository *Repository, addressRepository *address.Repository,
	validatorRepository *validator.Repository, coinRepository *coin.Repository, coinService *coin.Service, broadcastService *broadcast.Service) *Service {
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
	}
}

func (s Service) GetSaveTxJobChannel() chan []*models.Transaction {
	return s.jobSaveTxs
}
func (s Service) GetSaveTxsOutputJobChannel() chan []*models.Transaction {
	return s.jobSaveTxsOutput
}
func (s Service) GetSaveInvalidTxsJobChannel() chan []*models.InvalidTransaction {
	return s.jobSaveInvalidTxs
}
func (s Service) GetSaveTxValidatorJobChannel() chan []*models.TransactionValidator {
	return s.jobSaveValidatorTxs
}

//Handle response and save block to DB
func (s *Service) HandleTransactionsFromBlockResponse(blockHeight uint64, blockCreatedAt time.Time,
	transactions []responses.Transaction) error {

	var txList []*models.Transaction
	var invalidTxList []*models.InvalidTransaction

	for _, tx := range transactions {
		if tx.Log == nil {
			transaction, err := s.handleValidTransaction(tx, blockHeight, blockCreatedAt)
			if err != nil {
				return err
			}
			txList = append(txList, transaction)
		} else {
			transaction, err := s.handleInvalidTransaction(tx, blockHeight, blockCreatedAt)
			if err != nil {
				return err
			}
			invalidTxList = append(invalidTxList, transaction)
		}
	}

	if len(txList) > 0 {
		s.GetSaveTxJobChannel() <- txList
	}

	if len(invalidTxList) > 0 {
		s.GetSaveInvalidTxsJobChannel() <- invalidTxList
	}

	return nil
}

func (s *Service) SaveTransactionsWorker(jobs <-chan []*models.Transaction) {
	for transactions := range jobs {
		err := s.txRepository.SaveAll(transactions)
		helpers.HandleError(err)

		links, err := s.getLinksTxValidator(transactions)
		helpers.HandleError(err)
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

		//no need to publish a big number of transaction
		if len(transactions) > 10 {
			go s.broadcastService.PublishTransactions(transactions[:10])
		} else {
			go s.broadcastService.PublishTransactions(transactions)
		}
	}
}
func (s *Service) SaveTransactionsOutputWorker(jobs <-chan []*models.Transaction) {
	for transactions := range jobs {
		err := s.SaveAllTxOutputs(transactions)
		helpers.HandleError(err)
	}
}
func (s *Service) SaveInvalidTransactionsWorker(jobs <-chan []*models.InvalidTransaction) {
	for transactions := range jobs {
		err := s.txRepository.SaveAllInvalid(transactions)
		helpers.HandleError(err)
	}
}

func (s Service) SaveTxValidatorWorker(jobs <-chan []*models.TransactionValidator) {
	for links := range jobs {
		err := s.txRepository.LinkWithValidators(links)
		helpers.HandleError(err)
	}
}

func (s *Service) SaveAllTxOutputs(txList []*models.Transaction) error {
	var list []*models.TransactionOutput
	for _, tx := range txList {
		if tx.Type != models.TxTypeSend && tx.Type != models.TxTypeMultiSend {
			continue
		}
		if tx.ID == 0 {
			return errors.New("no transaction id")
		}
		if tx.Type == models.TxTypeSend {
			var txData models.SendTxData
			err := json.Unmarshal(tx.Data, &txData)
			if err != nil {
				return err
			}
			if txData.To == "" {
				return errors.New("empty receiver of transaction")
			}
			toId, err := s.addressRepository.FindId(helpers.RemovePrefix(txData.To))
			helpers.HandleError(err)
			coinID, err := s.coinRepository.FindIdBySymbol(txData.Coin)
			helpers.HandleError(err)
			list = append(list, &models.TransactionOutput{
				TransactionID: tx.ID,
				ToAddressID:   toId,
				CoinID:        coinID,
				Value:         txData.Value,
			})
		}
		if tx.Type == models.TxTypeMultiSend {
			var txData models.MultiSendTxData
			err := json.Unmarshal(tx.Data, &txData)
			if err != nil {
				return err
			}
			for _, receiver := range txData.List {
				toId, err := s.addressRepository.FindId(helpers.RemovePrefix(receiver.To))
				helpers.HandleError(err)
				coinID, err := s.coinRepository.FindIdBySymbol(receiver.Coin)
				helpers.HandleError(err)
				list = append(list, &models.TransactionOutput{
					TransactionID: tx.ID,
					ToAddressID:   toId,
					CoinID:        coinID,
					Value:         receiver.Value,
				})
			}
		}
	}
	// TODO: should to be chunked?
	if len(list) > 0 {
		err := s.txRepository.SaveAllTxOutputs(list)
		helpers.HandleError(err)
	}
	return nil
}

func (s *Service) handleValidTransaction(tx responses.Transaction, blockHeight uint64, blockCreatedAt time.Time) (*models.Transaction, error) {
	fromId, err := s.addressRepository.FindId(helpers.RemovePrefix(tx.From))
	if err != nil {
		return nil, err
	}
	nonce, err := strconv.ParseUint(tx.Nonce, 10, 64)
	if err != nil {
		return nil, err
	}
	gasPrice, err := strconv.ParseUint(tx.GasPrice, 10, 64)
	if err != nil {
		return nil, err
	}
	gas, err := strconv.ParseUint(tx.Gas, 10, 64)
	if err != nil {
		return nil, err
	}
	gasCoin, err := s.coinRepository.FindIdBySymbol(tx.GasCoin)
	if err != nil {
		return nil, err
	}
	txData, err := json.Marshal(*tx.Data)
	if err != nil {
		return nil, err
	}
	payload, err := base64.StdEncoding.DecodeString(tx.Payload)
	if err != nil {
		return nil, err
	}
	rawTxData := make([]byte, hex.DecodedLen(len(tx.RawTx)))
	rawTx, err := hex.Decode(rawTxData, []byte(tx.RawTx))
	if err != nil {
		return nil, err
	}
	transaction := &models.Transaction{
		FromAddressID: fromId,
		BlockID:       blockHeight,
		Nonce:         nonce,
		GasPrice:      gasPrice,
		Gas:           gas,
		GasCoinID:     gasCoin,
		CreatedAt:     blockCreatedAt,
		Type:          tx.Type,
		Hash:          helpers.RemovePrefix(tx.Hash),
		ServiceData:   tx.ServiceData,
		Data:          txData,
		Tags:          *tx.Tags,
		Payload:       payload,
		RawTx:         rawTxData[:rawTx],
	}

	if transaction.Type == models.TxTypeCreateCoin {
		err = s.coinService.CreateFromTx(tx)
		helpers.HandleError(err)
	}

	return transaction, nil
}

func (s *Service) handleInvalidTransaction(tx responses.Transaction, blockHeight uint64, blockCreatedAt time.Time) (*models.InvalidTransaction, error) {
	fromId, err := s.addressRepository.FindId(helpers.RemovePrefix(tx.From))
	if err != nil {
		return nil, err
	}
	invalidTxData, err := json.Marshal(tx)
	if err != nil {
		return nil, err
	}
	return &models.InvalidTransaction{
		FromAddressID: fromId,
		BlockID:       blockHeight,
		CreatedAt:     blockCreatedAt,
		Type:          tx.Type,
		Hash:          helpers.RemovePrefix(tx.Hash),
		TxData:        string(invalidTxData),
	}, nil
}

func (s Service) getLinksTxValidator(transactions []*models.Transaction) ([]*models.TransactionValidator, error) {
	var links []*models.TransactionValidator

	for _, tx := range transactions {
		if tx.ID == 0 {
			return nil, errors.New("no transaction id")
		}
		var (
			err         error
			validatorPk string
		)
		switch tx.Type {
		case models.TxTypeDeclareCandidacy:
			var txData models.DeclareCandidacyTxData
			err = json.Unmarshal(tx.Data, &txData)
			validatorPk = txData.PubKey
		case models.TxTypeDelegate:
			var txData models.DelegateTxData
			err = json.Unmarshal(tx.Data, &txData)
			validatorPk = txData.PubKey
		case models.TxTypeUnbound:
			var txData models.UnbondTxData
			err = json.Unmarshal(tx.Data, &txData)
			validatorPk = txData.PubKey
		case models.TxTypeSetCandidateOnline:
			var txData models.SetCandidateTxData
			err = json.Unmarshal(tx.Data, &txData)
			validatorPk = txData.PubKey
		case models.TxTypeSetCandidateOffline:
			var txData models.SetCandidateTxData
			err = json.Unmarshal(tx.Data, &txData)
			validatorPk = txData.PubKey
		case models.TxTypeEditCandidate:
			var txData models.EditCandidateTxData
			err = json.Unmarshal(tx.Data, &txData)
			validatorPk = txData.PubKey
		}
		if err != nil {
			return nil, err
		}

		if validatorPk != "" {
			validatorId, err := s.validatorRepository.FindIdByPkOrCreate(helpers.RemovePrefix(validatorPk))
			if err != nil {
				return nil, err
			}
			links = append(links, &models.TransactionValidator{
				TransactionID: tx.ID,
				ValidatorID:   validatorId,
			})
		}
	}

	return links, nil
}
