package liquidity_pool

import (
	"github.com/MinterTeam/minter-explorer-extender/v2/address"
	"github.com/MinterTeam/minter-go-sdk/v2/transaction"
	"github.com/MinterTeam/node-grpc-gateway/api_pb"
	"github.com/go-pg/pg/v10"
	"github.com/sirupsen/logrus"
	"math/big"
)

type Service struct {
	repository             *Repository
	addressRepository      *address.Repository
	logger                 *logrus.Entry
	jobUpdateLiquidityPool chan *api_pb.BlockResponse_Transaction
}

func (s *Service) UpdateLiquidityPoolWorker(jobs <-chan *api_pb.BlockResponse_Transaction) {
	for tx := range jobs {
		var err error
		switch transaction.Type(tx.Type) {
		case transaction.TypeBuySwapPool,
			transaction.TypeSellSwapPool,
			transaction.TypeSellAllSwapPool:
			err = s.updateVolumes(tx)
		case transaction.TypeAddSwapPool:
			err = s.addToLiquidityPool(tx)
		case transaction.TypeRemoveSwapPool:
			err = s.removeFromLiquidityPool(tx)
		}

		if err != nil {
			s.logger.Error(err)
		}
	}
}

func (s *Service) JobUpdateLiquidityPoolChannel() chan *api_pb.BlockResponse_Transaction {
	return s.jobUpdateLiquidityPool
}

func (s *Service) addToLiquidityPool(tx *api_pb.BlockResponse_Transaction) error {
	txData := new(api_pb.AddSwapPoolData)
	if err := tx.GetData().UnmarshalTo(txData); err != nil {
		return err
	}

	txTags := tx.GetTags()

	lp, err := s.repository.getLiquidityPoolByCoinIds(txData.Coin0.Id, txData.Coin1.Id)
	if err != nil && err != pg.ErrNoRows {
		return err
	}

	var firstCoinVolume, secondCoinVolume *big.Int

	if lp.FirstCoinVolume == "" {
		firstCoinVolume = big.NewInt(0)
	} else {
		firstCoinVolume, _ = big.NewInt(0).SetString(lp.FirstCoinVolume, 10)
	}

	if lp.SecondCoinVolume == "" {
		secondCoinVolume = big.NewInt(0)
	} else {
		secondCoinVolume, _ = big.NewInt(0).SetString(lp.SecondCoinVolume, 10)
	}

	volume0, _ := big.NewInt(0).SetString(txData.Volume0, 10)
	firstCoinVolume.Add(firstCoinVolume, volume0)

	volume1, _ := big.NewInt(0).SetString(txTags["tx.volume1"], 10)
	secondCoinVolume.Add(secondCoinVolume, volume1)

	lp.Liquidity = txTags["tx.liquidity"]
	lp.FirstCoinId = txData.Coin0.Id
	lp.SecondCoinId = txData.Coin1.Id

	return s.repository.UpdateLiquidityPool(lp)
}

func (s *Service) removeFromLiquidityPool(tx *api_pb.BlockResponse_Transaction) error {
	txData := new(api_pb.RemoveSwapPoolData)
	if err := tx.GetData().UnmarshalTo(txData); err != nil {
		return err
	}

	txTags := tx.GetTags()

	lp, err := s.repository.getLiquidityPoolByCoinIds(txData.Coin0.Id, txData.Coin1.Id)
	if err != nil && err != pg.ErrNoRows {
		return err
	}

	var firstCoinVolume, secondCoinVolume *big.Int

	if lp.FirstCoinVolume == "" {
		firstCoinVolume = big.NewInt(0)
	} else {
		firstCoinVolume, _ = big.NewInt(0).SetString(lp.FirstCoinVolume, 10)
	}

	if lp.SecondCoinVolume == "" {
		secondCoinVolume = big.NewInt(0)
	} else {
		secondCoinVolume, _ = big.NewInt(0).SetString(lp.SecondCoinVolume, 10)
	}

	volume0, _ := big.NewInt(0).SetString(txTags["tx.volume0"], 10)
	firstCoinVolume.Sub(firstCoinVolume, volume0)

	volume1, _ := big.NewInt(0).SetString(txTags["tx.volume1"], 10)
	secondCoinVolume.Sub(secondCoinVolume, volume1)

	lp.Liquidity = txData.Liquidity
	lp.FirstCoinId = txData.Coin0.Id
	lp.SecondCoinId = txData.Coin1.Id

	return s.repository.UpdateLiquidityPool(lp)
}

func (s *Service) updateVolumes(tx *api_pb.BlockResponse_Transaction) error {
	//todo
	return nil
}

func NewService(repository *Repository, addressRepository *address.Repository, logger *logrus.Entry) *Service {
	return &Service{
		repository:             repository,
		addressRepository:      addressRepository,
		logger:                 logger,
		jobUpdateLiquidityPool: make(chan *api_pb.BlockResponse_Transaction, 1),
	}
}
