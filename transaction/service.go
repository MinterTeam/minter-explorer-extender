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
	"github.com/daniildulin/minter-node-api/responses"
	"strconv"
)

type Service struct {
	txRepository      *Repository
	addressRepository *address.Repository
	coinRepository    *coin.Repository
	coinService       *coin.Service
}

func NewService(repository *Repository, addressRepository *address.Repository, coinRepository *coin.Repository, coinService *coin.Service) *Service {
	return &Service{
		txRepository:      repository,
		coinRepository:    coinRepository,
		addressRepository: addressRepository,
		coinService:       coinService,
	}
}

//Handle response and save block to DB
func (s *Service) HandleBlockResponse(response *responses.BlockResponse) error {
	var txList []*models.Transaction
	//var invalidTxList []*models.InvalidTransaction //TODO: don't forget about

	height, err := strconv.ParseUint(response.Result.Height, 10, 64)
	helpers.HandleError(err)

	for _, tx := range response.Result.Transactions {
		txFrom := []rune(tx.From)
		fromId, err := s.addressRepository.FindId(string(txFrom[2:]))
		helpers.HandleError(err)
		hash := []rune(tx.Hash)
		nonce, err := strconv.ParseUint(tx.Nonce, 10, 64)
		helpers.HandleError(err)
		gasPrice, err := strconv.ParseUint(tx.GasPrice, 10, 64)
		helpers.HandleError(err)
		gas, err := strconv.ParseUint(tx.Gas, 10, 64)
		helpers.HandleError(err)
		gasCoin, err := s.coinRepository.FindIdBySymbol(tx.GasCoin)
		helpers.HandleError(err)
		txData, err := json.Marshal(*tx.Data)
		helpers.HandleError(err)
		payload, err := base64.StdEncoding.DecodeString(tx.Payload)
		helpers.HandleError(err)
		rawTxData := make([]byte, hex.DecodedLen(len(tx.RawTx)))
		rawTx, err := hex.Decode(rawTxData, []byte(tx.RawTx))
		helpers.HandleError(err)
		if tx.Log == nil {
			t := &models.Transaction{
				FromAddressID: fromId,
				BlockID:       height,
				Nonce:         nonce,
				GasPrice:      gasPrice,
				Gas:           gas,
				GasCoinID:     gasCoin,
				CreatedAt:     response.Result.Time,
				Type:          tx.Type,
				Hash:          string(hash[2:]),
				ServiceData:   tx.ServiceData,
				Data:          txData,
				Tags:          *tx.Tags,
				Payload:       payload,
				RawTx:         rawTxData[:rawTx],
			}
			txList = append(txList, t)
			if t.Type == models.TxTypeCreateCoin {
				err = s.coinService.CreateFromTx(tx)
				helpers.HandleError(err)
			}
		}
	}
	err = s.txRepository.SaveAll(txList)
	helpers.HandleError(err)
	err = s.SaveAllTxOutputs(txList)
	helpers.HandleError(err)
	return err
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
			txTo := []rune(txData.To)
			toId, err := s.addressRepository.FindId(string(txTo[2:]))
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
			for _, receiver := range txData {
				txTo := []rune(receiver.Address)
				toId, err := s.addressRepository.FindId(string(txTo[2:]))
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
		return s.txRepository.SaveAllTxOutputs(list)
	}
	return nil
}
