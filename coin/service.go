package coin

import (
	"errors"
	"fmt"
	"github.com/MinterTeam/minter-explorer-extender/v2/address"
	"github.com/MinterTeam/minter-explorer-extender/v2/env"
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/MinterTeam/minter-explorer-tools/v4/helpers"
	"github.com/MinterTeam/minter-go-sdk/v2/api/grpc_client"
	"github.com/MinterTeam/minter-go-sdk/v2/transaction"
	"github.com/MinterTeam/node-grpc-gateway/api_pb"
	"github.com/go-pg/pg/v10"
	"github.com/go-resty/resty/v2"
	"github.com/sirupsen/logrus"
	"math/big"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	env                   *env.ExtenderEnvironment
	httpClient            *resty.Client
	nodeApi               *grpc_client.Client
	Storage               *Repository
	addressRepository     *address.Repository
	logger                *logrus.Entry
	jobUpdateCoins        chan []*models.Transaction
	jobUpdateCoinsFromMap chan map[uint64]struct{}
}

func NewService(env *env.ExtenderEnvironment, nodeApi *grpc_client.Client, repository *Repository,
	addressRepository *address.Repository, logger *logrus.Entry) *Service {
	return &Service{
		env:                   env,
		httpClient:            resty.New(),
		nodeApi:               nodeApi,
		Storage:               repository,
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

func (s Service) HandleCoinsFromBlock(block *api_pb.BlockResponse) error {
	var coins []*models.Coin
	var err error
	var height uint64

	for _, tx := range block.Transactions {
		if tx.Log != "" || tx.Code > 0 {
			continue
		}

		height = tx.Height

		switch transaction.Type(tx.Type) {
		case transaction.TypeCreateCoin, transaction.TypeCreateToken:
			coin, err := s.ExtractFromTx(tx, block.Height)
			if err != nil {
				return err
			}
			coins = append(coins, coin)
		case transaction.TypeRecreateCoin:
			txData := new(api_pb.RecreateCoinData)
			tx.GetData()
			if err = tx.GetData().UnmarshalTo(txData); err != nil {
				return err
			}

			coinId, err := strconv.ParseUint(tx.GetTags()["tx.coin_id"], 10, 64)
			if err != nil {
				return err
			}

			newCoin := &models.Coin{
				ID:               uint(coinId),
				Crr:              uint(txData.ConstantReserveRatio),
				Name:             txData.Name,
				Type:             models.CoinTypeBase,
				Volume:           txData.InitialAmount,
				Reserve:          txData.InitialReserve,
				Symbol:           txData.Symbol,
				MaxSupply:        txData.MaxSupply,
				Burnable:         false,
				Mintable:         false,
				CreatedAtBlockId: uint(height),
				Version:          0,
			}

			err = s.RecreateCoin(newCoin)
			if err != nil {
				return err
			}
		case transaction.TypeRecreateToken:
			txData := new(api_pb.RecreateTokenData)
			tx.GetData()
			if err = tx.GetData().UnmarshalTo(txData); err != nil {
				return err
			}

			coinId, err := strconv.ParseUint(tx.GetTags()["tx.coin_id"], 10, 64)
			if err != nil {
				return err
			}

			newCoin := &models.Coin{
				ID:               uint(coinId),
				Crr:              uint(0),
				Type:             models.CoinTypeToken,
				Name:             txData.Name,
				Volume:           txData.InitialAmount,
				Reserve:          "0",
				Symbol:           txData.Symbol,
				MaxSupply:        txData.MaxSupply,
				Burnable:         txData.Burnable,
				Mintable:         txData.Mintable,
				CreatedAtBlockId: uint(height),
				Version:          0,
			}

			err = s.RecreateCoin(newCoin)
			if err != nil {
				return err
			}
		case transaction.TypeMintToken:
			err = s.MintToken(tx)
		case transaction.TypeBurnToken:
			err = s.BurnToken(tx)
		}
	}

	if len(coins) > 0 {
		err = s.CreateNewCoins(coins)
	}

	return err
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
			Type:             models.CoinTypeBase,
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
			Type:             models.CoinTypeToken,
			Volume:           txData.InitialAmount,
			MaxSupply:        txData.MaxSupply,
			Name:             txData.Name,
			Symbol:           txData.Symbol,
			Reserve:          "0",
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
	err := s.Storage.SaveAllNewIfNotExist(coins)
	return err
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
		return s.Storage.UpdateAll(coins)
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

	coin, err := s.Storage.GetById(uint(coinId))
	if err != nil && err != pg.ErrNoRows {
		return nil, err
	}

	ownerAddressId := uint(0)
	if coinResp.OwnerAddress != nil {
		ownerAddressId, err = s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(coinResp.OwnerAddress.Value))
	}

	symbol := strings.Split(coinResp.Symbol, "-")

	if symbol[0] == "LP" {
		coin.Symbol = coinResp.Symbol
	} else {
		coin.Symbol = symbol[0]
	}

	coin.Name = coinResp.Name
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

	return s.Storage.UpdateOwnerBySymbol(symbol, id)
}

func (s *Service) RecreateCoin(newCoin *models.Coin) error {
	coins, err := s.Storage.GetCoinBySymbol(newCoin.Symbol)
	if err != nil {
		return err
	}

	for _, c := range coins {
		if c.Version == 0 {
			c.Version = uint(len(coins))
			err = s.Storage.Update(&c)
			if err != nil {
				return err
			}
			newCoin.OwnerAddressId = c.OwnerAddressId
			break
		}
	}
	s.Storage.RemoveFromCacheBySymbol(newCoin.Symbol)
	err = s.Storage.Add(newCoin)
	return err
}

func (s *Service) RecreateToken(data *api_pb.RecreateTokenData, txTags map[string]string, height uint64) error {
	coins, err := s.Storage.GetCoinBySymbol(data.Symbol)
	if err != nil {
		return err
	}
	coinId, err := strconv.ParseUint(txTags["tx.coin_id"], 10, 64)
	if err != nil {
		return err
	}
	newCoin := &models.Coin{
		ID:               uint(coinId),
		Type:             coins[0].Type,
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
			err = s.Storage.Update(&c)
			if err != nil {
				return err
			}
			newCoin.OwnerAddressId = c.OwnerAddressId
			break
		}
	}
	s.Storage.RemoveFromCacheBySymbol(data.Symbol)
	err = s.Storage.Add(newCoin)
	return err
}

func (s *Service) CreatePoolToken(tx *api_pb.TransactionResponse) (*models.Coin, error) {

	txTags := tx.GetTags()
	coinId, err := strconv.ParseUint(txTags["tx.pool_token_id"], 10, 64)
	if err != nil {
		return nil, err
	}

	volume := big.NewInt(1000)
	txLiquidity, _ := big.NewInt(0).SetString(txTags["tx.liquidity"], 10)

	volume.Add(volume, txLiquidity)

	c := &models.Coin{
		ID:               uint(coinId),
		Type:             models.CoinTypePoolToken,
		Name:             fmt.Sprintf("Liquidity Pool %s", txTags["tx.pair_ids"]),
		Symbol:           txTags["tx.pool_token"],
		Volume:           volume.String(),
		Crr:              0,
		Reserve:          "",
		MaxSupply:        "1000000000000000000000000000000000",
		Version:          0,
		Burnable:         false,
		Mintable:         false,
		CreatedAtBlockId: uint(tx.Height),
		CreatedAt:        time.Now(),
	}

	err = s.Storage.Add(c)

	return c, err
}

func (s *Service) GetBySymbolAndVersion(symbol string, version uint) (*models.Coin, error) {
	list, err := s.Storage.GetCoinBySymbol(symbol)
	if err != nil {
		return nil, err
	}

	for _, c := range list {
		if c.Version == version {
			return &c, nil
		}
	}

	return nil, errors.New("coin not found")
}

func (s *Service) MintToken(tx *api_pb.TransactionResponse) error {
	txData := new(api_pb.MintTokenData)
	if err := tx.GetData().UnmarshalTo(txData); err != nil {
		return err
	}

	c, err := s.Storage.GetById(uint(txData.Coin.Id))
	if err != nil {
		return err
	}

	coinVolume, _ := big.NewInt(0).SetString(c.Volume, 10)
	addVolume, _ := big.NewInt(0).SetString(txData.Value, 10)

	coinVolume.Add(coinVolume, addVolume)

	c.Volume = coinVolume.String()

	_, err = s.Storage.DB.Model(c).WherePK().Update()

	return err
}

func (s *Service) BurnToken(tx *api_pb.TransactionResponse) error {
	txData := new(api_pb.BurnTokenData)
	if err := tx.GetData().UnmarshalTo(txData); err != nil {
		return err
	}

	c, err := s.Storage.GetById(uint(txData.Coin.Id))
	if err != nil {
		return err
	}

	coinVolume, _ := big.NewInt(0).SetString(c.Volume, 10)
	burnVolume, _ := big.NewInt(0).SetString(txData.Value, 10)

	coinVolume.Sub(coinVolume, burnVolume)

	c.Volume = coinVolume.String()

	_, err = s.Storage.DB.Model(c).WherePK().Update()

	return err
}
