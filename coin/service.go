package coin

import (
	"encoding/json"
	"errors"
	"github.com/MinterTeam/minter-explorer-extender/address"
	"github.com/MinterTeam/minter-explorer-tools/helpers"
	"github.com/MinterTeam/minter-explorer-tools/models"
	"github.com/daniildulin/minter-node-api"
	"github.com/daniildulin/minter-node-api/responses"
	"github.com/sirupsen/logrus"
	"strconv"
	"time"
)

type Service struct {
	env               *models.ExtenderEnvironment
	nodeApi           *minter_node_api.MinterNodeApi
	repository        *Repository
	addressRepository *address.Repository
	logger            *logrus.Entry
	jobUpdateCoins    chan []*models.Transaction
}

func NewService(env *models.ExtenderEnvironment, nodeApi *minter_node_api.MinterNodeApi, repository *Repository,
	addressRepository *address.Repository, logger *logrus.Entry) *Service {
	return &Service{
		env:               env,
		nodeApi:           nodeApi,
		repository:        repository,
		addressRepository: addressRepository,
		logger:            logger,
		jobUpdateCoins:    make(chan []*models.Transaction, 1),
	}
}

type CreateCoinData struct {
	Name           string `json:"name"`
	Symbol         string `json:"symbol"`
	InitialAmount  string `json:"initial_amount"`
	InitialReserve string `json:"initial_reserve"`
	Crr            string `json:"crr"`
}

func (s *Service) GetUpdateCoinsFromTxsJobChannel() chan []*models.Transaction {
	return s.jobUpdateCoins
}

func (s Service) ExtractCoinsFromTransactions(transactions []responses.Transaction) ([]*models.Coin, error) {
	var coins []*models.Coin
	for _, tx := range transactions {
		if tx.Type == models.TxTypeCreateCoin {
			coin, err := s.ExtractFromTx(tx)
			if err != nil {
				s.logger.Error(err)
				return nil, err
			}
			coins = append(coins, coin)
		}
	}
	return coins, nil
}

func (s *Service) ExtractFromTx(tx responses.Transaction) (*models.Coin, error) {
	if tx.Data == nil {
		s.logger.Warn("empty transaction data")
		return nil, errors.New("no data for creating a coin")
	}
	var txData models.CreateCoinTxData
	jsonData, err := json.Marshal(*tx.Data)
	if err != nil {
		s.logger.Error(err)
		return nil, err
	}
	err = json.Unmarshal(jsonData, &txData)
	if err != nil {
		s.logger.Error(err)
		return nil, err
	}
	fromId, err := s.addressRepository.FindId(helpers.RemovePrefix(tx.From))
	if err != nil {
		s.logger.Error(err)
		return nil, err
	}
	crr, err := strconv.ParseUint(txData.ConstantReserveRatio, 10, 64)
	if err != nil {
		s.logger.Error(err)
		return nil, err
	}
	return &models.Coin{
		CreationAddressID: &fromId,
		Crr:               crr,
		Volume:            txData.InitialAmount,
		ReserveBalance:    txData.InitialReserve,
		Name:              txData.Name,
		Symbol:            txData.Symbol,
		DeletedAt:         nil,
	}, nil
}

func (s *Service) CreateNewCoins(coins []*models.Coin) error {
	err := s.repository.SaveAllIfNotExist(coins)
	if err != nil {
		s.logger.Error(err)
	}
	return err
}

func (s *Service) UpdateCoinsInfoFromTxsWorker(jobs <-chan []*models.Transaction) {
	for transactions := range jobs {
		coinsMap := make(map[string]struct{})
		// Find coins in transaction for update
		for _, tx := range transactions {
			symbol, err := s.repository.FindSymbolById(tx.GasCoinID)
			if err != nil {
				s.logger.Error(err)
				continue
			}
			if symbol != s.env.BaseCoin {
				coinsMap[symbol] = struct{}{}
			}
			switch tx.Type {
			case models.TxTypeSellCoin:
				var txData models.SellCoinTxData
				err := json.Unmarshal(tx.Data, &txData)
				if err != nil {
					s.logger.Error(err)
				}
				coinsMap[txData.CoinToBuy] = struct{}{}
				coinsMap[txData.CoinToSell] = struct{}{}
			case models.TxTypeBuyCoin:
				var txData models.BuyCoinTxData
				err := json.Unmarshal(tx.Data, &txData)
				if err != nil {
					s.logger.Error(err)
				}
				coinsMap[txData.CoinToBuy] = struct{}{}
				coinsMap[txData.CoinToSell] = struct{}{}
			case models.TxTypeSellAllCoin:
				var txData models.SellAllCoinTxData
				err := json.Unmarshal(tx.Data, &txData)
				if err != nil {
					s.logger.Error(err)
				}
				coinsMap[txData.CoinToBuy] = struct{}{}
				coinsMap[txData.CoinToSell] = struct{}{}
			}
		}
		delete(coinsMap, s.env.BaseCoin)
		if len(coinsMap) > 0 {
			coinsForUpdate := make([]string, len(coinsMap))
			i := 0
			for symbol := range coinsMap {
				coinsForUpdate[i] = symbol
				i++
			}
			err := s.UpdateCoinsInfo(coinsForUpdate)
			if err != nil {
				s.logger.Error(err)
			}
		}
	}
}

func (s *Service) UpdateCoinsInfo(symbols []string) error {
	var coins []*models.Coin
	for _, symbol := range symbols {
		if symbol == s.env.BaseCoin {
			continue
		}
		coin, err := s.GetCoinFromNode(symbol)
		if err != nil {
			s.logger.Error(err)
			continue
		}
		coins = append(coins, coin)
	}
	if len(coins) > 0 {
		return s.repository.SaveAllIfNotExist(coins)
	}
	return nil
}

func (s *Service) GetCoinFromNode(symbol string) (*models.Coin, error) {
	coinResp, err := s.nodeApi.GetCoinInfo(symbol)
	if err != nil {
		s.logger.Error(err)
		return nil, err
	}
	now := time.Now()
	coin := new(models.Coin)
	id, err := s.repository.FindIdBySymbol(symbol)
	if err != nil {
		s.logger.Error(err)
		return nil, err
	}
	coin.ID = id
	if coinResp.Error != nil {
		return nil, errors.New(coinResp.Error.Message)
	}
	crr, err := strconv.ParseUint(coinResp.Result.Crr, 10, 64)
	if err != nil {
		s.logger.Error(err)
		return nil, err
	}
	coin.Name = coinResp.Result.Name
	coin.Symbol = coinResp.Result.Symbol
	coin.Crr = crr
	coin.ReserveBalance = coinResp.Result.ReserveBalance
	coin.Volume = coinResp.Result.Volume
	coin.DeletedAt = nil
	coin.UpdatedAt = now
	return coin, nil
}
