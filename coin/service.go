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
}

func NewService(env *models.ExtenderEnvironment, nodeApi *minter_node_api.MinterNodeApi, repository *Repository,
	addressRepository *address.Repository, logger *logrus.Entry) *Service {
	return &Service{
		env:               env,
		nodeApi:           nodeApi,
		repository:        repository,
		addressRepository: addressRepository,
		logger:            logger,
	}
}

type CreateCoinData struct {
	Name           string `json:"name"`
	Symbol         string `json:"symbol"`
	InitialAmount  string `json:"initial_amount"`
	InitialReserve string `json:"initial_reserve"`
	Crr            string `json:"crr"`
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
	}, nil
}

func (s *Service) CreateNewCoins(coins []*models.Coin) error {
	err := s.repository.SaveAllIfNotExist(coins)
	if err != nil {
		s.logger.Error(err)

	}
	return err
}

func (s *Service) UpdateAllCoinsInfoWorker() {
	for {
		err := s.UpdateAllCoinsInfo()
		helpers.HandleError(err)
		time.Sleep(time.Duration(s.env.CoinsUpdateTime) * time.Minute)
	}
}

func (s *Service) UpdateAllCoinsInfo() error {
	coins, err := s.repository.GetAllCoins()
	if err != nil {
		return err
	}
	for _, coin := range coins {
		if coin.Symbol == s.env.BaseCoin {
			continue
		}
		err = s.UpdateCoinInfo(coin)
		if err != nil {
			return err
		}
		err = s.repository.db.Update(coin)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) UpdateCoinInfo(coin *models.Coin) error {
	coinResp, err := s.nodeApi.GetCoinInfo(coin.Symbol)
	if err != nil {
		return err
	}
	now := time.Now()
	if coinResp.Error != nil && coinResp.Error.Code == 404 {
		coin.DeletedAt = &now
		coin.CreationAddressID = nil
		coin.CreationTransactionID = nil
	} else {
		crr, err := strconv.ParseUint(coinResp.Result.Crr, 10, 64)
		if err != nil {
			return err
		}
		coin.Name = coinResp.Result.Name
		coin.Crr = crr
		coin.ReserveBalance = coinResp.Result.ReserveBalance
		coin.Volume = coinResp.Result.Volume
		coin.DeletedAt = nil
		coin.UpdatedAt = now
	}
	return nil
}
