package transaction

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/MinterTeam/minter-explorer-extender/address"
	"github.com/MinterTeam/minter-explorer-extender/coin"
	"github.com/MinterTeam/minter-explorer-extender/helpers"
	"github.com/MinterTeam/minter-explorer-extender/models"
	"github.com/MinterTeam/minter-explorer-extender/validator"
	"github.com/daniildulin/minter-node-api/responses"
	"math"
	"strconv"
	"sync"
	"time"
)

type Service struct {
	env                 *models.ExtenderEnvironment
	txRepository        *Repository
	addressRepository   *address.Repository
	validatorRepository *validator.Repository
	coinRepository      *coin.Repository
	coinService         *coin.Service
}

func NewService(env *models.ExtenderEnvironment, repository *Repository, addressRepository *address.Repository, validatorRepository *validator.Repository,
	coinRepository *coin.Repository, coinService *coin.Service) *Service {
	return &Service{
		env:                 env,
		txRepository:        repository,
		coinRepository:      coinRepository,
		addressRepository:   addressRepository,
		coinService:         coinService,
		validatorRepository: validatorRepository,
	}
}

//Handle response and save block to DB
func (s *Service) HandleTransactionsFromBlockResponse(blockHeight uint64, blockCreatedAt time.Time,
	transactions []responses.Transaction, validators []*models.Validator) error {

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
		err := s.txRepository.SaveAll(txList)
		helpers.HandleError(err)
		err = s.SaveAllTxOutputs(txList, s.env.TxChunkSize)
		helpers.HandleError(err)
		err = s.LinkWithValidators(txList, validators)
		helpers.HandleError(err)
	}
	if len(invalidTxList) > 0 {
		err := s.txRepository.SaveAllInvalid(invalidTxList)
		helpers.HandleError(err)
	}

	return nil
}

func (s *Service) SaveAllTxOutputs(txList []*models.Transaction, chunkSize int) error {
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
	if len(list) > 0 {
		chunksCount := int(math.Ceil(float64(len(list)) / float64(chunkSize)))
		chunks := make([][]*models.TransactionOutput, chunksCount)
		for i := 0; i < chunksCount; i++ {
			start := chunkSize * i
			end := start + chunkSize
			if end > len(list) {
				end = len(list)
			}
			chunks[i] = list[start:end]
		}

		var wg sync.WaitGroup
		wg.Add(len(chunks))
		go func() {
			defer wg.Done()
			err := s.txRepository.SaveAllTxOutputs(list)
			helpers.HandleError(err)
		}()
		wg.Wait()
	}
	return nil
}

func (s Service) LinkWithValidators(transactions []*models.Transaction, validators []*models.Validator) error {
	var links []*models.TransactionValidator
	for _, tx := range transactions {
		for _, vld := range validators {
			// if validator has been saved not in current block ID = 0
			validatorId, err := s.validatorRepository.FindIdByPk(vld.PublicKey)
			if err != nil {
				return err
			}
			links = append(links, &models.TransactionValidator{
				TransactionID: tx.ID,
				ValidatorID:   validatorId,
			})
		}
	}
	if len(links) <= 0 {
		return nil
	}

	chunksCount := int(math.Ceil(float64(len(links)) / float64(s.env.TxChunkSize)))
	chunks := make([][]*models.TransactionValidator, chunksCount)
	for i := 0; i < chunksCount; i++ {
		start := s.env.TxChunkSize * i
		end := start + s.env.TxChunkSize
		if end > len(links) {
			end = len(links)
		}
		chunks[i] = links[start:end]
	}

	var wg sync.WaitGroup
	wg.Add(len(chunks))
	for _, links := range chunks {
		go func() {
			defer wg.Done()
			err := s.txRepository.LinkWithValidators(links)
			helpers.HandleError(err)
		}()
	}
	wg.Wait()
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
