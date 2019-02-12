package transaction

import (
	"encoding/json"
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
	if response.Result.TxCount == "0" {
		return nil
	}
	var txList []*models.Transaction

	height, err := strconv.ParseUint(response.Result.Height, 10, 64)
	helpers.HandleError(err)

	for _, tx := range response.Result.Transactions {
		if tx.Type == models.TxTypeCreateCoin {
			err = s.coinService.CreateFromTx(tx)
		}
		txFrom := []rune(tx.From)
		fromId, err := s.addressRepository.FindIdOrCreate(string(txFrom[2:]))
		helpers.HandleError(err)
		hash := []rune(tx.From)
		nonce, err := strconv.ParseUint(tx.Nonce, 10, 64)
		helpers.HandleError(err)
		gasPrice, err := strconv.ParseUint(tx.GasPrice, 10, 64)
		helpers.HandleError(err)
		gas, err := strconv.ParseUint(tx.Gas, 10, 64)
		helpers.HandleError(err)
		gasCoin, err := s.coinRepository.FindIdBySymbol(tx.GasCoin)
		helpers.HandleError(err)
		rawTx, err := json.Marshal(tx)
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
				Data:          *tx.Data,
				Tags:          *tx.Tags,
				Payload:       []byte(tx.Payload),
				RawTx:         rawTx,
			}
			txList = append(txList, t)
		}
	}
	return s.txRepository.SaveAll(txList)
}
