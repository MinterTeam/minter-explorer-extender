package coin

import (
	"github.com/MinterTeam/minter-explorer-extender/v2/address"
	"github.com/MinterTeam/minter-explorer-extender/v2/env"
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/MinterTeam/minter-explorer-tools/v4/helpers"
	"github.com/MinterTeam/minter-go-sdk/v2/api/grpc_client"
	"github.com/MinterTeam/minter-go-sdk/v2/transaction"
	"github.com/MinterTeam/node-grpc-gateway/api_pb"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/anypb"
)

type Service struct {
	env                   *env.ExtenderEnvironment
	nodeApi               *grpc_client.Client
	repository            *Repository
	addressRepository     *address.Repository
	logger                *logrus.Entry
	jobUpdateCoins        chan []*models.Transaction
	jobUpdateCoinsFromMap chan map[uint64]struct{}
	lastCoinId            uint
}

func NewService(env *env.ExtenderEnvironment, nodeApi *grpc_client.Client, repository *Repository, addressRepository *address.Repository, logger *logrus.Entry) *Service {
	coinId, err := repository.GetLastCoinId()
	if err != nil {
		logger.Fatal(err)
	}

	return &Service{
		env:                   env,
		nodeApi:               nodeApi,
		repository:            repository,
		addressRepository:     addressRepository,
		logger:                logger,
		jobUpdateCoins:        make(chan []*models.Transaction, 1),
		jobUpdateCoinsFromMap: make(chan map[uint64]struct{}, 1),
		lastCoinId:            coinId,
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
	s.UpdateCoinIdCache()
	for _, tx := range block.Transactions {

		if transaction.Type(tx.Type) == transaction.TypeCreateCoin {
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

			err := s.RecreateCoin(txData, block.Height)
			if err != nil {
				s.logger.Fatal(err)
			}
		}
	}
	return coins, nil
}

func (s *Service) ExtractFromTx(tx *api_pb.BlockResponse_Transaction, blockId uint64) (*models.Coin, error) {
	var txData = new(api_pb.CreateCoinData)
	err := tx.Data.UnmarshalTo(txData)
	if err != nil {
		return nil, err
	}

	s.lastCoinId += 1

	coin := &models.Coin{
		ID:               s.lastCoinId,
		Crr:              uint(txData.ConstantReserveRatio),
		Volume:           txData.InitialAmount,
		Reserve:          txData.InitialReserve,
		MaxSupply:        txData.MaxSupply,
		Name:             txData.Name,
		Symbol:           txData.Symbol,
		CreatedAtBlockId: uint(blockId),
		Version:          0,
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
	coinResp, err := s.nodeApi.CoinInfoByID(coinId, optionalHeight...)
	if err != nil {
		return nil, err
	}

	coin, err := s.repository.GetById(uint(coinId))
	if err != nil && err.Error() != "pg: no rows in result set" {
		return nil, err
	}

	ownerAddressId := uint(0)
	if coinResp.OwnerAddress != nil {
		ownerAddressId, err = s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(coinResp.OwnerAddress.Value))
	}

	coin.Name = coinResp.Name
	coin.Symbol = coinResp.Symbol
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

func (s *Service) RecreateCoin(data *api_pb.RecreateCoinData, height uint64) error {
	coins, err := s.repository.GetCoinBySymbol(data.Symbol)
	s.lastCoinId += 1
	newCoin := &models.Coin{
		ID:               s.lastCoinId,
		Crr:              uint(data.ConstantReserveRatio),
		Name:             data.Name,
		Volume:           data.InitialAmount,
		Reserve:          data.InitialReserve,
		Symbol:           data.Symbol,
		MaxSupply:        data.MaxSupply,
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

func (s *Service) UpdateCoinIdCache() {
	coinId, err := s.repository.GetLastCoinId()
	if err != nil {
		s.logger.Fatal(err)
	}
	s.lastCoinId = coinId
}
