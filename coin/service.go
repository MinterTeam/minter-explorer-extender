package coin

import (
	"fmt"
	"github.com/MinterTeam/minter-explorer-extender/v2/address"
	"github.com/MinterTeam/minter-explorer-extender/v2/env"
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/MinterTeam/minter-explorer-tools/v4/helpers"
	"github.com/MinterTeam/minter-go-sdk/v2/api/grpc_client"
	"github.com/MinterTeam/minter-go-sdk/v2/transaction"
	"github.com/MinterTeam/node-grpc-gateway/api_pb"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/anypb"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	env                   *env.ExtenderEnvironment
	nodeApi               *grpc_client.Client
	repository            *Repository
	addressRepository     *address.Repository
	logger                *logrus.Entry
	jobUpdateCoins        chan []*models.Transaction
	jobUpdateCoinsFromMap chan map[uint64]struct{}
}

func NewService(env *env.ExtenderEnvironment, nodeApi *grpc_client.Client, repository *Repository, addressRepository *address.Repository, logger *logrus.Entry) *Service {
	return &Service{
		env:                   env,
		nodeApi:               nodeApi,
		repository:            repository,
		addressRepository:     addressRepository,
		logger:                logger,
		jobUpdateCoins:        make(chan []*models.Transaction, 1),
		jobUpdateCoinsFromMap: make(chan map[uint64]struct{}, 1),
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

func (s *Service) GetUpdateCoinsFromCoinsMapJobChannel() chan map[uint64]struct{} {
	return s.jobUpdateCoinsFromMap
}

func (s Service) ExtractCoinsFromTransactions(block *api_pb.BlockResponse) ([]*models.Coin, error) {
	var coins []*models.Coin
	for _, tx := range block.Transactions {

		if tx.Log != "" || tx.Code > 0 {
			continue
		}

		if transaction.Type(tx.Type) == transaction.TypeCreateCoin || transaction.Type(tx.Type) == transaction.TypeCreateToken {
			coin, err := s.ExtractFromTx(tx, block.Height)
			if err != nil {
				return nil, err
			}
			coins = append(coins, coin)
		}
		if transaction.Type(tx.Type) == transaction.TypeRecreateCoin {
			txData := new(api_pb.RecreateCoinData)
			tx.GetData()

			if err := tx.GetData().UnmarshalTo(txData); err != nil {
				s.logger.Fatal(err)
			}

			err := s.RecreateCoin(txData, tx.GetTags(), block.Height)
			if err != nil {
				s.logger.Fatal(err)
			}
		}
	}
	return coins, nil
}

func (s *Service) ExtractFromTx(tx *api_pb.TransactionResponse, blockId uint64) (*models.Coin, error) {
	var coin = new(models.Coin)

	txTags := tx.GetTags()
	coinId, err := strconv.ParseUint(txTags["tx.coin_id"], 10, 64)
	if err != nil {
		return nil, err
	}

	switch transaction.Type(tx.Type) {
	case transaction.TypeCreateCoin:
		var txData = new(api_pb.CreateCoinData)
		err := tx.Data.UnmarshalTo(txData)
		if err != nil {
			return nil, err
		}
		coin = &models.Coin{
			ID:               uint(coinId),
			Crr:              uint(txData.ConstantReserveRatio),
			Volume:           txData.InitialAmount,
			Reserve:          txData.InitialReserve,
			MaxSupply:        txData.MaxSupply,
			Name:             txData.Name,
			Symbol:           txData.Symbol,
			CreatedAtBlockId: uint(blockId),
			Burnable:         false,
			Mintable:         false,
			Version:          0,
		}
	case transaction.TypeCreateToken:
		var txData = new(api_pb.CreateTokenData)
		err := tx.Data.UnmarshalTo(txData)
		if err != nil {
			return nil, err
		}
		coin = &models.Coin{
			ID:               uint(coinId),
			Volume:           txData.InitialAmount,
			MaxSupply:        txData.MaxSupply,
			Name:             txData.Name,
			Symbol:           txData.Symbol,
			CreatedAtBlockId: uint(blockId),
			Burnable:         txData.Mintable,
			Mintable:         txData.Mintable,
			Version:          0,
		}
	}

	fromId, err := s.addressRepository.FindId(helpers.RemovePrefix(tx.From))

	if err != nil {
		s.logger.Error(err)
	} else {
		coin.OwnerAddressId = fromId
	}

	return coin, nil
}

func (s *Service) CreateNewCoins(coins []*models.Coin) error {
	err := s.repository.SaveAllNewIfNotExist(coins)
	return err
}

func (s *Service) UpdateCoinsInfoFromTxsWorker(jobs <-chan []*models.Transaction) {
	for transactions := range jobs {
		coinsMap := make(map[uint64]struct{})
		// Find coins in transaction for update
		for _, tx := range transactions {

			coinsMap[tx.GasCoinID] = struct{}{}

			switch transaction.Type(tx.Type) {
			case transaction.TypeSellCoin:
				txData := new(api_pb.SellCoinData)
				if err := tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
					s.logger.Error(err)
					continue
				}
				coinsMap[txData.CoinToBuy.Id] = struct{}{}
				coinsMap[txData.CoinToSell.Id] = struct{}{}
			case transaction.TypeBuyCoin:
				txData := new(api_pb.BuyCoinData)
				if err := tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
					s.logger.Error(err)
					continue
				}
				coinsMap[txData.CoinToBuy.Id] = struct{}{}
				coinsMap[txData.CoinToSell.Id] = struct{}{}
			case transaction.TypeSellAllCoin:
				txData := new(api_pb.SellAllCoinData)
				if err := tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
					s.logger.Error(err)
					continue
				}
				coinsMap[txData.CoinToBuy.Id] = struct{}{}
				coinsMap[txData.CoinToSell.Id] = struct{}{}
			}
		}
		s.GetUpdateCoinsFromCoinsMapJobChannel() <- coinsMap
	}
}

func (s Service) UpdateCoinsInfoFromCoinsMap(job <-chan map[uint64]struct{}) {
	for coinsMap := range job {
		delete(coinsMap, 0)
		if len(coinsMap) > 0 {
			coinsForUpdate := make([]uint64, len(coinsMap))
			i := 0
			for coinId := range coinsMap {
				coinsForUpdate[i] = coinId
				i++
			}
			err := s.UpdateCoinsInfo(coinsForUpdate)
			if err != nil {
				s.logger.Error(err)
			}
		}
	}
}

func (s *Service) UpdateCoinsInfo(coinIds []uint64) error {
	var coins []*models.Coin
	for _, coinId := range coinIds {
		coin, err := s.GetCoinFromNode(coinId)
		if err != nil {
			s.logger.Error(err)
			continue
		}
		coins = append(coins, coin)
	}
	if len(coins) > 0 {
		return s.repository.UpdateAll(coins)
	}
	return nil
}

func (s *Service) GetCoinFromNode(coinId uint64, optionalHeight ...uint64) (*models.Coin, error) {
	start := time.Now()
	coinResp, err := s.nodeApi.CoinInfoByID(coinId, optionalHeight...)
	if err != nil {
		return nil, err
	}
	elapsed := time.Since(start)
	s.logger.Info(fmt.Sprintf("Coin: %d Coin's data getting time: %s", coinId, elapsed))

	coin, err := s.repository.GetById(uint(coinId))
	if err != nil && err.Error() != "pg: no rows in result set" {
		return nil, err
	}

	ownerAddressId := uint(0)
	if coinResp.OwnerAddress != nil {
		ownerAddressId, err = s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(coinResp.OwnerAddress.Value))
	}

	symbol := strings.Split(coinResp.Symbol, "-")

	coin.Name = coinResp.Name
	coin.Symbol = symbol[0]
	coin.Crr = uint(coinResp.Crr)
	coin.Reserve = coinResp.ReserveBalance
	coin.Volume = coinResp.Volume
	coin.MaxSupply = coinResp.MaxSupply
	coin.OwnerAddressId = ownerAddressId

	return coin, nil
}

func (s *Service) ChangeOwner(symbol, owner string) error {
	id, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(owner))
	if err != nil {
		return err
	}

	return s.repository.UpdateOwnerBySymbol(symbol, id)
}

func (s *Service) RecreateCoin(data *api_pb.RecreateCoinData, txTags map[string]string, height uint64) error {
	coins, err := s.repository.GetCoinBySymbol(data.Symbol)
	if err != nil {
		return err
	}

	coinId, err := strconv.ParseUint(txTags["tx.coin_id"], 10, 64)
	if err != nil {
		return err
	}

	newCoin := &models.Coin{
		ID:               uint(coinId),
		Crr:              uint(data.ConstantReserveRatio),
		Name:             data.Name,
		Volume:           data.InitialAmount,
		Reserve:          data.InitialReserve,
		Symbol:           data.Symbol,
		MaxSupply:        data.MaxSupply,
		Burnable:         false,
		Mintable:         false,
		CreatedAtBlockId: uint(height),
		Version:          0,
	}

	for _, c := range coins {
		if c.Version == 0 {
			c.Version = uint(len(coins))
			err = s.repository.Update(&c)
			if err != nil {
				return err
			}
			newCoin.OwnerAddressId = c.OwnerAddressId
			break
		}
	}
	s.repository.RemoveFromCacheBySymbol(data.Symbol)
	err = s.repository.Add(newCoin)
	return err
}
func (s *Service) RecreateToken(data *api_pb.RecreateTokenData, txTags map[string]string, height uint64) error {
	coins, err := s.repository.GetCoinBySymbol(data.Symbol)
	if err != nil {
		return err
	}
	coinId, err := strconv.ParseUint(txTags["tx.coin_id"], 10, 64)
	if err != nil {
		return err
	}
	newCoin := &models.Coin{
		ID:               uint(coinId),
		Name:             data.Name,
		Volume:           data.InitialAmount,
		Symbol:           data.Symbol,
		MaxSupply:        data.MaxSupply,
		CreatedAtBlockId: uint(height),
		Burnable:         data.Burnable,
		Mintable:         data.Mintable,
		Version:          0,
	}

	for _, c := range coins {
		if c.Version == 0 {
			c.Version = uint(len(coins))
			err = s.repository.Update(&c)
			if err != nil {
				return err
			}
			newCoin.OwnerAddressId = c.OwnerAddressId
			break
		}
	}
	s.repository.RemoveFromCacheBySymbol(data.Symbol)
	err = s.repository.Add(newCoin)
	return err
}

func (s *Service) CreatePoolToken(tx *api_pb.TransactionResponse) error {

	txTags := tx.GetTags()
	coinId, err := strconv.ParseUint(txTags["tx.pool_token_id"], 10, 64)
	if err != nil {
		return err
	}

	c := &models.Coin{
		ID:        uint(coinId),
		Name:      txTags["tx.pool_token"],
		Symbol:    txTags["tx.pool_token"],
		Volume:    txTags["tx.liquidity"],
		Crr:       0,
		Reserve:   "",
		MaxSupply: "",
		Version:   0,
		Burnable:  false,
		Mintable:  false,
		CreatedAt: time.Now(),
	}

	var list []*models.Coin
	list = append(list, c)

	return s.CreateNewCoins(list)
}
