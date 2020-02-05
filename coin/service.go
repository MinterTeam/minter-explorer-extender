package coin

import (
	"errors"
	"github.com/MinterTeam/minter-explorer-extender/address"
	"github.com/MinterTeam/minter-explorer-extender/env"
	"github.com/MinterTeam/minter-explorer-tools/v4/helpers"
	"github.com/MinterTeam/minter-explorer-tools/v4/models"
	"github.com/MinterTeam/minter-go-sdk/api"
	"github.com/MinterTeam/minter-go-sdk/transaction"
	"github.com/sirupsen/logrus"
	"strconv"
	"time"
)

type Service struct {
	env                   *env.ExtenderEnvironment
	nodeApi               *api.Api
	repository            *Repository
	addressRepository     *address.Repository
	logger                *logrus.Entry
	jobUpdateCoins        chan []*models.Transaction
	jobUpdateCoinsFromMap chan map[string]struct{}
}

func NewService(env *env.ExtenderEnvironment, nodeApi *api.Api, repository *Repository,
	addressRepository *address.Repository, logger *logrus.Entry) *Service {
	return &Service{
		env:                   env,
		nodeApi:               nodeApi,
		repository:            repository,
		addressRepository:     addressRepository,
		logger:                logger,
		jobUpdateCoins:        make(chan []*models.Transaction, 1),
		jobUpdateCoinsFromMap: make(chan map[string]struct{}, 1),
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

func (s *Service) GetUpdateCoinsFromCoinsMapJobChannel() chan map[string]struct{} {
	return s.jobUpdateCoinsFromMap
}

func (s Service) ExtractCoinsFromTransactions(transactions []api.TransactionResult) ([]*models.Coin, error) {
	var coins []*models.Coin
	for _, tx := range transactions {
		if transaction.Type(tx.Type) == transaction.TypeCreateCoin {
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

func (s *Service) ExtractFromTx(tx api.TransactionResult) (*models.Coin, error) {
	if tx.Data == nil {
		s.logger.Warn("empty transaction data")
		return nil, errors.New("no data for creating a coin")
	}
	var txData transaction.CreateCoinData
	err := tx.Data.FillStruct(txData)
	coin := &models.Coin{
		Crr:            uint64(txData.ConstantReserveRatio),
		Volume:         txData.InitialAmount.String(),
		ReserveBalance: txData.InitialReserve.String(),
		Name:           txData.Name,
		Symbol:         string(txData.Symbol[:]),
		DeletedAt:      nil,
	}

	fromId, err := s.addressRepository.FindId(helpers.RemovePrefix(tx.From))
	if err != nil {
		s.logger.Error(err)
	} else {
		coin.CreationAddressID = &fromId
	}

	return coin, nil
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
			coinsMap[symbol] = struct{}{}
			switch transaction.Type(tx.Type) {
			case transaction.TypeSellCoin:
				coinsMap[tx.IData.(models.SellCoinTxData).CoinToBuy] = struct{}{}
				coinsMap[tx.IData.(models.SellCoinTxData).CoinToSell] = struct{}{}
			case transaction.TypeBuyCoin:
				coinsMap[tx.IData.(models.BuyCoinTxData).CoinToBuy] = struct{}{}
				coinsMap[tx.IData.(models.BuyCoinTxData).CoinToSell] = struct{}{}
			case transaction.TypeSellAllCoin:
				coinsMap[tx.IData.(models.SellAllCoinTxData).CoinToBuy] = struct{}{}
				coinsMap[tx.IData.(models.SellAllCoinTxData).CoinToSell] = struct{}{}
			}
		}
		s.GetUpdateCoinsFromCoinsMapJobChannel() <- coinsMap
	}
}

func (s Service) UpdateCoinsInfoFromCoinsMap(job <-chan map[string]struct{}) {
	for coinsMap := range job {
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
	coinResp, err := s.nodeApi.CoinInfo(symbol, 0)
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

	crr, err := strconv.ParseUint(coinResp.Crr, 10, 64)
	if err != nil {
		s.logger.Error(err)
		return nil, err
	}
	coin.Name = coinResp.Name
	coin.Symbol = coinResp.Symbol
	coin.Crr = crr
	coin.ReserveBalance = coinResp.ReserveBalance
	coin.Volume = coinResp.Volume
	coin.DeletedAt = nil
	coin.UpdatedAt = now
	return coin, nil
}
