package liquidity_pool

import (
	"github.com/MinterTeam/minter-explorer-extender/v2/address"
	"github.com/MinterTeam/minter-explorer-tools/v4/helpers"
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
	jobUpdateLiquidityPool chan *api_pb.TransactionResponse
}

func (s *Service) UpdateLiquidityPoolWorker(jobs <-chan *api_pb.TransactionResponse) {
	for tx := range jobs {
		var err error
		switch transaction.Type(tx.Type) {
		case transaction.TypeBuySwapPool:
			err = s.updateVolumesBuySwapPool(tx)
		case transaction.TypeSellSwapPool:
		case transaction.TypeSellAllSwapPool:
		case transaction.TypeAddLiquidity:
			err = s.addToLiquidityPool(tx)
		case transaction.TypeRemoveLiquidity:
			err = s.removeFromLiquidityPool(tx)
		}

		if err != nil {
			s.logger.Error(err)
		}
	}
}

func (s *Service) JobUpdateLiquidityPoolChannel() chan *api_pb.TransactionResponse {
	return s.jobUpdateLiquidityPool
}

func (s *Service) addToLiquidityPool(tx *api_pb.TransactionResponse) error {
	txData := new(api_pb.AddLiquidityData)
	if err := tx.GetData().UnmarshalTo(txData); err != nil {
		return err
	}

	txTags := tx.GetTags()

	var (
		firstCoinId, secondCoinId   uint64
		firstCoinVol, secondCoinVol string
	)

	if txData.Coin0.Id < txData.Coin1.Id {
		firstCoinId = txData.Coin0.Id
		firstCoinVol = txData.Volume0
		secondCoinId = txData.Coin1.Id
		secondCoinVol = txTags["tx.volume1"]
	} else {
		firstCoinId = txData.Coin1.Id
		firstCoinVol = txTags["tx.volume1"]
		secondCoinId = txData.Coin0.Id
		secondCoinVol = txData.Volume0
	}

	lp, err := s.repository.getLiquidityPoolByCoinIds(firstCoinId, secondCoinId)
	if err != nil && err != pg.ErrNoRows {
		return err
	}

	var firstCoinVolume, secondCoinVolume, liquidity, addressLiquidity *big.Int

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

	if lp.Liquidity == "" {
		liquidity = big.NewInt(0)
	} else {
		liquidity, _ = big.NewInt(0).SetString(lp.Liquidity, 10)
	}

	fcVolume, _ := big.NewInt(0).SetString(firstCoinVol, 10)
	firstCoinVolume.Add(firstCoinVolume, fcVolume)

	scVolume, _ := big.NewInt(0).SetString(secondCoinVol, 10)
	secondCoinVolume.Add(secondCoinVolume, scVolume)

	txLiquidity, _ := big.NewInt(0).SetString(txTags["tx.liquidity"], 10)
	liquidity.Add(liquidity, txLiquidity)

	lp.Liquidity = liquidity.String()
	lp.FirstCoinId = firstCoinId
	lp.SecondCoinId = secondCoinId
	lp.FirstCoinVolume = firstCoinVolume.String()
	lp.SecondCoinVolume = secondCoinVolume.String()

	err = s.repository.UpdateLiquidityPool(lp)
	if err != nil {
		return err
	}

	addressId, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(tx.From))
	if err != nil {
		return err
	}

	alp, err := s.repository.GetAddressLiquidityPool(addressId, lp.Id)
	if err != nil && err != pg.ErrNoRows {
		return err
	}

	if alp.Liquidity == "" {
		addressLiquidity = big.NewInt(0)
	} else {
		addressLiquidity, _ = big.NewInt(0).SetString(alp.Liquidity, 10)
	}

	addressLiquidity.Add(addressLiquidity, txLiquidity)
	alp.AddressId = uint64(addressId)
	alp.LiquidityPoolId = lp.Id
	alp.Liquidity = addressLiquidity.String()

	return s.repository.UpdateAddressLiquidityPool(alp)
}

func (s *Service) removeFromLiquidityPool(tx *api_pb.TransactionResponse) error {
	txData := new(api_pb.RemoveLiquidityData)
	if err := tx.GetData().UnmarshalTo(txData); err != nil {
		return err
	}

	txTags := tx.GetTags()

	var (
		firstCoinId, secondCoinId   uint64
		firstCoinVol, secondCoinVol string
	)

	if txData.Coin0.Id < txData.Coin1.Id {
		firstCoinId = txData.Coin0.Id
		firstCoinVol = txTags["tx.volume0"]
		secondCoinId = txData.Coin1.Id
		secondCoinVol = txTags["tx.volume1"]
	} else {
		firstCoinId = txData.Coin1.Id
		firstCoinVol = txTags["tx.volume1"]
		secondCoinId = txData.Coin0.Id
		secondCoinVol = txTags["tx.volume0"]
	}

	lp, err := s.repository.getLiquidityPoolByCoinIds(firstCoinId, secondCoinId)
	if err != nil && err != pg.ErrNoRows {
		return err
	}

	var firstCoinVolume, secondCoinVolume, liquidity *big.Int

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

	if lp.Liquidity == "" {
		liquidity = big.NewInt(0)
	} else {
		liquidity, _ = big.NewInt(0).SetString(lp.Liquidity, 10)
	}

	fcVolume, _ := big.NewInt(0).SetString(firstCoinVol, 10)
	firstCoinVolume.Sub(firstCoinVolume, fcVolume)

	scVolume, _ := big.NewInt(0).SetString(secondCoinVol, 10)
	secondCoinVolume.Sub(secondCoinVolume, scVolume)

	txLiquidity, _ := big.NewInt(0).SetString(txData.Liquidity, 10)
	liquidity.Sub(liquidity, txLiquidity)

	lp.Liquidity = liquidity.String()
	lp.FirstCoinId = firstCoinId
	lp.SecondCoinId = secondCoinId
	lp.FirstCoinVolume = firstCoinVolume.String()
	lp.SecondCoinVolume = secondCoinVolume.String()

	err = s.repository.UpdateLiquidityPool(lp)
	if err != nil {
		return err
	}

	addressId, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(tx.From))
	if err != nil {
		return err
	}

	alp, err := s.repository.GetAddressLiquidityPool(addressId, lp.Id)
	if err != nil {
		return err
	}

	addressLiquidity, _ := big.NewInt(0).SetString(alp.Liquidity, 10)
	addressLiquidity.Sub(addressLiquidity, liquidity)

	alp.AddressId = uint64(addressId)
	alp.LiquidityPoolId = lp.Id
	alp.Liquidity = addressLiquidity.String()

	if addressLiquidity.Cmp(big.NewInt(0)) == 0 {
		return s.repository.DeleteAddressLiquidityPool(addressId, lp.Id)
	} else {
		return s.repository.UpdateAddressLiquidityPool(alp)
	}
}

func (s *Service) updateVolumesBuySwapPool(tx *api_pb.TransactionResponse) error {

	txData := new(api_pb.BuySwapPoolData)
	if err := tx.GetData().UnmarshalTo(txData); err != nil {
		return err
	}

	txTags := tx.GetTags()

	var (
		firstCoinId, secondCoinId   uint64
		firstCoinVol, secondCoinVol string
	)

	if txData.CoinToBuy.Id < txData.CoinToSell.Id {
		firstCoinId = txData.CoinToBuy.Id
		firstCoinVol = txData.ValueToBuy
		secondCoinId = txData.CoinToSell.Id
		secondCoinVol = txTags["tx.return"]
	} else {
		firstCoinId = txData.CoinToSell.Id
		firstCoinVol = txTags["tx.return"]
		secondCoinId = txData.CoinToBuy.Id
		secondCoinVol = txData.ValueToBuy
	}

	lp, err := s.repository.getLiquidityPoolByCoinIds(firstCoinId, secondCoinId)
	if err != nil {
		return err
	}

	lpFirstCoinVol, _ := big.NewInt(0).SetString(lp.FirstCoinVolume, 10)
	txFirstCoinVol, _ := big.NewInt(0).SetString(firstCoinVol, 10)

	lpSecondCoinVol, _ := big.NewInt(0).SetString(lp.SecondCoinVolume, 10)
	txSecondCoinVol, _ := big.NewInt(0).SetString(secondCoinVol, 10)

	if txData.CoinToBuy.Id < txData.CoinToSell.Id {
		lpFirstCoinVol.Sub(lpFirstCoinVol, txFirstCoinVol)
		lpSecondCoinVol.Add(lpSecondCoinVol, txSecondCoinVol)
	} else {
		lpFirstCoinVol.Add(lpFirstCoinVol, txFirstCoinVol)
		lpSecondCoinVol.Sub(lpSecondCoinVol, txSecondCoinVol)
	}

	lp.FirstCoinVolume = lpFirstCoinVol.String()
	lp.SecondCoinVolume = lpSecondCoinVol.String()

	return s.repository.UpdateLiquidityPool(lp)
}

func NewService(repository *Repository, addressRepository *address.Repository, logger *logrus.Entry) *Service {
	return &Service{
		repository:             repository,
		addressRepository:      addressRepository,
		logger:                 logger,
		jobUpdateLiquidityPool: make(chan *api_pb.TransactionResponse, 1),
	}
}
