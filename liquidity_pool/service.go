package liquidity_pool

import (
	"github.com/MinterTeam/minter-explorer-extender/v2/address"
	"github.com/MinterTeam/minter-explorer-extender/v2/balance"
	"github.com/MinterTeam/minter-explorer-extender/v2/coin"
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/MinterTeam/minter-explorer-tools/v4/helpers"
	"github.com/MinterTeam/minter-go-sdk/v2/transaction"
	"github.com/MinterTeam/node-grpc-gateway/api_pb"
	"github.com/go-pg/pg/v10"
	"github.com/sirupsen/logrus"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

type Service struct {
	repository             *Repository
	addressRepository      *address.Repository
	coinService            *coin.Service
	balanceService         *balance.Service
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
			err = s.updateVolumesSellSwapPool(tx)
		case transaction.TypeSellAllSwapPool:
			err = s.updateVolumesSellAllSwapPool(tx)
		case transaction.TypeAddLiquidity:
			err = s.addToLiquidityPool(tx)
		case transaction.TypeCreateSwapPool:
			err = s.createLiquidityPool(tx)
		case transaction.TypeRemoveLiquidity:
			err = s.removeFromLiquidityPool(tx)
		case transaction.TypeSend, transaction.TypeMultisend:
			err = s.updateAddressPoolVolumes(tx)
		default:
			err = s.updateVolumesByCommission(tx)
		}

		if err != nil {
			s.logger.Error(err)
		}
	}
}

func (s *Service) JobUpdateLiquidityPoolChannel() chan *api_pb.TransactionResponse {
	return s.jobUpdateLiquidityPool
}

func (s *Service) createLiquidityPool(tx *api_pb.TransactionResponse) error {
	txData := new(api_pb.CreateSwapPoolData)
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
		secondCoinVol = txData.Volume1
	} else {
		firstCoinId = txData.Coin1.Id
		firstCoinVol = txData.Volume1
		secondCoinId = txData.Coin0.Id
		secondCoinVol = txData.Volume0
	}

	_, err := s.coinService.CreatePoolToken(tx)
	if err != nil {
		return err
	}

	_, err = s.addToPool(firstCoinId, secondCoinId, firstCoinVol, secondCoinVol, helpers.RemovePrefix(tx.From), txTags)
	if err != nil {
		return err
	}

	s.balanceService.GetAddressesChannel() <- models.BlockAddresses{
		Height:    tx.Height,
		Addresses: []string{helpers.RemovePrefix(tx.From)},
	}

	//TODO: temporary disabled
	//var re = regexp.MustCompile(`(?mi)p-\d+`)
	//if re.MatchString(txData.Coin0.Symbol) {
	//	fromAddressId, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(tx.From))
	//	if err != nil {
	//		return err
	//	}
	//	err = s.updateAddressPoolVolumesWhenCreate(fromAddressId, lp.Id, txData.Volume0)
	//}
	//if re.MatchString(txData.Coin1.Symbol) {
	//	fromAddressId, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(tx.From))
	//	if err != nil {
	//		return err
	//	}
	//	err = s.updateAddressPoolVolumesWhenCreate(fromAddressId, lp.Id, txData.Volume1)
	//}

	return err
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

	_, err := s.addToPool(firstCoinId, secondCoinId, firstCoinVol, secondCoinVol, helpers.RemovePrefix(tx.From), txTags)
	if err != nil {
		return err
	}

	s.balanceService.GetAddressesChannel() <- models.BlockAddresses{
		Height:    tx.Height,
		Addresses: []string{helpers.RemovePrefix(tx.From)},
	}

	txLiquidity, _ := big.NewInt(0).SetString(txTags["tx.liquidity"], 10)
	coinId, err := strconv.ParseUint(txTags["tx.pool_token_id"], 10, 64)
	if err != nil {
		return err
	}
	c, err := s.coinService.Repository.GetById(uint(coinId))
	if err != nil {
		return err
	}
	coinLiquidity, _ := big.NewInt(0).SetString(c.Volume, 10)
	coinLiquidity.Add(coinLiquidity, txLiquidity)
	c.Volume = coinLiquidity.String()
	err = s.coinService.Repository.Update(c)
	if err != nil {
		return err
	}

	return err
}

func (s *Service) addToPool(firstCoinId, secondCoinId uint64, firstCoinVol, secondCoinVol, txFrom string,
	txTags map[string]string) (*models.LiquidityPool, error) {

	lp, err := s.repository.getLiquidityPoolByCoinIds(firstCoinId, secondCoinId)
	if err != nil && err != pg.ErrNoRows {
		return nil, err
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
		liquidity = big.NewInt(1000)
	} else {
		liquidity, _ = big.NewInt(0).SetString(lp.Liquidity, 10)
	}

	fcVolume, _ := big.NewInt(0).SetString(firstCoinVol, 10)
	firstCoinVolume.Add(firstCoinVolume, fcVolume)

	scVolume, _ := big.NewInt(0).SetString(secondCoinVol, 10)
	secondCoinVolume.Add(secondCoinVolume, scVolume)

	txLiquidity, _ := big.NewInt(0).SetString(txTags["tx.liquidity"], 10)
	liquidity.Add(liquidity, txLiquidity)

	txPoolToken := strings.Split(txTags["tx.pool_token"], "-")

	lpId, err := strconv.ParseUint(txPoolToken[1], 10, 64)
	if err != nil {
		return nil, err
	}

	coinId, err := strconv.ParseUint(txTags["tx.pool_token_id"], 10, 64)
	if err != nil {
		return nil, err
	}

	lp.Id = lpId
	lp.TokenId = coinId
	lp.Liquidity = liquidity.String()
	lp.FirstCoinId = firstCoinId
	lp.SecondCoinId = secondCoinId
	lp.FirstCoinVolume = firstCoinVolume.String()
	lp.SecondCoinVolume = secondCoinVolume.String()

	err = s.repository.UpdateLiquidityPool(lp)
	if err != nil {
		return nil, err
	}

	addressId, err := s.addressRepository.FindIdOrCreate(txFrom)
	if err != nil {
		return nil, err
	}

	alp, err := s.repository.GetAddressLiquidityPool(addressId, lp.Id)
	if err != nil && err != pg.ErrNoRows {
		return nil, err
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

	err = s.repository.UpdateAddressLiquidityPool(alp)
	if err != nil {
		return nil, err
	}

	return lp, err
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
	addressLiquidity.Sub(addressLiquidity, txLiquidity)

	alp.AddressId = uint64(addressId)
	alp.LiquidityPoolId = lp.Id
	alp.Liquidity = addressLiquidity.String()

	s.balanceService.GetAddressesChannel() <- models.BlockAddresses{
		Height:    tx.Height,
		Addresses: []string{helpers.RemovePrefix(tx.From)},
	}

	coinId, err := strconv.ParseUint(txTags["tx.pool_token_id"], 10, 64)
	if err != nil {
		return err
	}
	c, err := s.coinService.Repository.GetById(uint(coinId))
	if err != nil {
		return err
	}
	coinLiquidity, _ := big.NewInt(0).SetString(c.Volume, 10)
	coinLiquidity.Sub(coinLiquidity, txLiquidity)
	c.Volume = coinLiquidity.String()
	err = s.coinService.Repository.Update(c)
	if err != nil {
		return err
	}

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

	list, err := s.getPoolChainFromTags(txTags)
	if err != nil {
		return err
	}

	for _, poolData := range list {

		coinId0, err := strconv.ParseUint(poolData[0]["coinId"], 10, 64)
		if err != nil {
			return err
		}
		coinId1, err := strconv.ParseUint(poolData[1]["coinId"], 10, 64)
		if err != nil {
			return err
		}

		if coinId0 < coinId1 {
			firstCoinId = coinId0
			firstCoinVol = poolData[0]["volume"]
			secondCoinId = coinId1
			secondCoinVol = poolData[1]["volume"]
		} else {
			firstCoinId = coinId1
			firstCoinVol = poolData[1]["volume"]
			secondCoinId = coinId0
			secondCoinVol = poolData[0]["volume"]
		}

		lp, err := s.repository.getLiquidityPoolByCoinIds(firstCoinId, secondCoinId)
		if err != nil {
			return err
		}

		lpFirstCoinVol, _ := big.NewInt(0).SetString(lp.FirstCoinVolume, 10)
		txFirstCoinVol, _ := big.NewInt(0).SetString(firstCoinVol, 10)

		lpSecondCoinVol, _ := big.NewInt(0).SetString(lp.SecondCoinVolume, 10)
		txSecondCoinVol, _ := big.NewInt(0).SetString(secondCoinVol, 10)

		if coinId0 > coinId1 {
			lpFirstCoinVol.Sub(lpFirstCoinVol, txFirstCoinVol)
			lpSecondCoinVol.Add(lpSecondCoinVol, txSecondCoinVol)
		} else {
			lpFirstCoinVol.Add(lpFirstCoinVol, txFirstCoinVol)
			lpSecondCoinVol.Sub(lpSecondCoinVol, txSecondCoinVol)
		}

		lp.FirstCoinVolume = lpFirstCoinVol.String()
		lp.SecondCoinVolume = lpSecondCoinVol.String()

		err = s.repository.UpdateLiquidityPool(lp)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) updateVolumesSellSwapPool(tx *api_pb.TransactionResponse) error {

	txData := new(api_pb.SellSwapPoolData)
	if err := tx.GetData().UnmarshalTo(txData); err != nil {
		return err
	}

	txTags := tx.GetTags()

	var (
		firstCoinId, secondCoinId   uint64
		firstCoinVol, secondCoinVol string
	)

	list, err := s.getPoolChainFromTags(txTags)
	if err != nil {
		return err
	}

	for _, poolData := range list {

		coinId0, err := strconv.ParseUint(poolData[0]["coinId"], 10, 64)
		if err != nil {
			return err
		}
		coinId1, err := strconv.ParseUint(poolData[1]["coinId"], 10, 64)
		if err != nil {
			return err
		}

		if coinId0 < coinId1 {
			firstCoinId = coinId0
			firstCoinVol = poolData[0]["volume"]
			secondCoinId = coinId1
			secondCoinVol = poolData[1]["volume"]
		} else {
			firstCoinId = coinId1
			firstCoinVol = poolData[1]["volume"]
			secondCoinId = coinId0
			secondCoinVol = poolData[0]["volume"]
		}

		lp, err := s.repository.getLiquidityPoolByCoinIds(firstCoinId, secondCoinId)
		if err != nil {
			return err
		}

		lpFirstCoinVol, _ := big.NewInt(0).SetString(lp.FirstCoinVolume, 10)
		txFirstCoinVol, _ := big.NewInt(0).SetString(firstCoinVol, 10)

		lpSecondCoinVol, _ := big.NewInt(0).SetString(lp.SecondCoinVolume, 10)
		txSecondCoinVol, _ := big.NewInt(0).SetString(secondCoinVol, 10)

		if coinId0 < coinId1 {
			lpFirstCoinVol.Sub(lpFirstCoinVol, txFirstCoinVol)
			lpSecondCoinVol.Add(lpSecondCoinVol, txSecondCoinVol)
		} else {
			lpFirstCoinVol.Add(lpFirstCoinVol, txFirstCoinVol)
			lpSecondCoinVol.Sub(lpSecondCoinVol, txSecondCoinVol)
		}

		lp.FirstCoinVolume = lpFirstCoinVol.String()
		lp.SecondCoinVolume = lpSecondCoinVol.String()

		err = s.repository.UpdateLiquidityPool(lp)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) updateVolumesSellAllSwapPool(tx *api_pb.TransactionResponse) error {

	txData := new(api_pb.SellAllSwapPoolData)
	if err := tx.GetData().UnmarshalTo(txData); err != nil {
		return err
	}

	txTags := tx.GetTags()

	var (
		firstCoinId, secondCoinId   uint64
		firstCoinVol, secondCoinVol string
	)

	list, err := s.getPoolChainFromTags(txTags)
	if err != nil {
		return err
	}

	for _, poolData := range list {

		coinId0, err := strconv.ParseUint(poolData[0]["coinId"], 10, 64)
		if err != nil {
			return err
		}
		coinId1, err := strconv.ParseUint(poolData[1]["coinId"], 10, 64)
		if err != nil {
			return err
		}

		if coinId0 < coinId1 {
			firstCoinId = coinId0
			firstCoinVol = poolData[0]["volume"]
			secondCoinId = coinId1
			secondCoinVol = poolData[1]["volume"]
		} else {
			firstCoinId = coinId1
			firstCoinVol = poolData[1]["volume"]
			secondCoinId = coinId0
			secondCoinVol = poolData[0]["volume"]
		}

		lp, err := s.repository.getLiquidityPoolByCoinIds(firstCoinId, secondCoinId)
		if err != nil {
			return err
		}

		lpFirstCoinVol, _ := big.NewInt(0).SetString(lp.FirstCoinVolume, 10)
		txFirstCoinVol, _ := big.NewInt(0).SetString(firstCoinVol, 10)

		lpSecondCoinVol, _ := big.NewInt(0).SetString(lp.SecondCoinVolume, 10)
		txSecondCoinVol, _ := big.NewInt(0).SetString(secondCoinVol, 10)

		if coinId0 < coinId1 {
			lpFirstCoinVol.Sub(lpFirstCoinVol, txFirstCoinVol)
			lpSecondCoinVol.Add(lpSecondCoinVol, txSecondCoinVol)
		} else {
			lpFirstCoinVol.Add(lpFirstCoinVol, txFirstCoinVol)
			lpSecondCoinVol.Sub(lpSecondCoinVol, txSecondCoinVol)
		}

		lp.FirstCoinVolume = lpFirstCoinVol.String()
		lp.SecondCoinVolume = lpSecondCoinVol.String()

		err = s.repository.UpdateLiquidityPool(lp)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) updateVolumesByCommission(tx *api_pb.TransactionResponse) error {
	tags := tx.GetTags()
	if tx.GasCoin.Id == 0 || tags["tx.commission_conversion"] != "pool" {
		return nil
	}

	lp, err := s.repository.getLiquidityPoolByCoinIds(0, tx.GasCoin.Id)
	if err != nil {
		return err
	}

	bipCommission, _ := big.NewInt(0).SetString(tags["tx.commission_in_base_coin"], 10)
	coinCommission, _ := big.NewInt(0).SetString(tags["tx.commission_amount"], 10)

	lpFirstCoinVol, _ := big.NewInt(0).SetString(lp.FirstCoinVolume, 10)
	lpSecondCoinVol, _ := big.NewInt(0).SetString(lp.SecondCoinVolume, 10)

	lpFirstCoinVol.Sub(lpFirstCoinVol, bipCommission)
	lpSecondCoinVol.Add(lpSecondCoinVol, coinCommission)

	lp.FirstCoinVolume = lpFirstCoinVol.String()
	lp.SecondCoinVolume = lpSecondCoinVol.String()

	return s.repository.UpdateLiquidityPool(lp)
}

func (s *Service) GetPoolByPairString(pair string) (*models.LiquidityPool, error) {
	ids := strings.Split(pair, "-")
	firstCoinId, err := strconv.ParseUint(ids[0], 10, 64)
	if err != nil {
		return nil, err
	}
	secondCoinId, err := strconv.ParseUint(ids[1], 10, 64)
	if err != nil {
		return nil, err
	}
	if firstCoinId < secondCoinId {
		return s.repository.getLiquidityPoolByCoinIds(firstCoinId, secondCoinId)
	} else {
		return s.repository.getLiquidityPoolByCoinIds(secondCoinId, firstCoinId)
	}
}

func (s *Service) GetPoolsByTxTags(tags map[string]string) ([]models.LiquidityPool, error) {
	pools, err := s.getPoolChainFromTags(tags)
	if err != nil {
		return nil, err
	}
	var idList []uint64
	for id, _ := range pools {
		idList = append(idList, id)
	}
	return s.repository.GetAllByIds(idList)
}

func (s *Service) updateAddressPoolVolumes(tx *api_pb.TransactionResponse) error {
	var err error

	fromAddressId, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(tx.From))
	if err != nil {
		return err
	}

	switch transaction.Type(tx.Type) {
	case transaction.TypeSend:
		txData := new(api_pb.SendData)
		if err = tx.Data.UnmarshalTo(txData); err != nil {
			return err
		}

		var re = regexp.MustCompile(`(?mi)p-\d+`)
		if re.MatchString(txData.Coin.Symbol) {
			err = s.updateAddressPoolVolumesByTxData(fromAddressId, txData)
		}

	case transaction.TypeMultisend:
		txData := new(api_pb.MultiSendData)
		if err = tx.Data.UnmarshalTo(txData); err != nil {
			return err
		}
		for _, data := range txData.List {
			var re = regexp.MustCompile(`(?mi)p-\d+`)
			if re.MatchString(data.Coin.Symbol) {
				err = s.updateAddressPoolVolumesByTxData(fromAddressId, data)
				if err != nil {
					return err
				}
			}
		}
	}

	return err
}

func (s *Service) updateAddressPoolVolumesByTxData(fromAddressId uint, txData *api_pb.SendData) error {
	toAddressId, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(txData.To))
	if err != nil {
		return err
	}

	lp, err := s.repository.getLiquidityPoolByTokenId(txData.Coin.Id)
	if err != nil {
		return err
	}
	alpFrom, err := s.repository.GetAddressLiquidityPool(fromAddressId, lp.Id)
	if err != nil {
		return err
	}
	txValue, _ := big.NewInt(0).SetString(txData.Value, 10)
	addressFromLiquidity, _ := big.NewInt(0).SetString(alpFrom.Liquidity, 10)
	addressFromLiquidity.Sub(addressFromLiquidity, txValue)
	alpFrom.Liquidity = addressFromLiquidity.String()

	alpTo, err := s.repository.GetAddressLiquidityPool(toAddressId, lp.Id)
	if err != nil && err != pg.ErrNoRows {
		return err
	} else if err != nil && err == pg.ErrNoRows {
		alpTo = &models.AddressLiquidityPool{
			LiquidityPoolId: lp.Id,
			AddressId:       uint64(toAddressId),
			Liquidity:       txData.Value,
		}
	} else {
		addressToLiquidity, _ := big.NewInt(0).SetString(alpTo.Liquidity, 10)
		addressToLiquidity.Add(addressToLiquidity, txValue)
		alpTo.Liquidity = addressFromLiquidity.String()
	}

	return s.repository.UpdateAllLiquidityPool([]*models.AddressLiquidityPool{alpFrom, alpTo})
}

func (s *Service) updateAddressPoolVolumesWhenCreate(fromAddressId uint, lpId uint64, value string) error {
	alpFrom, err := s.repository.GetAddressLiquidityPool(fromAddressId, lpId)
	if err != nil {
		return err
	}
	txValue, _ := big.NewInt(0).SetString(value, 10)
	//delta := big.NewInt(1000)
	//txValue.Sub(txValue, delta)

	addressFromLiquidity, _ := big.NewInt(0).SetString(alpFrom.Liquidity, 10)
	addressFromLiquidity.Sub(addressFromLiquidity, txValue)
	alpFrom.Liquidity = addressFromLiquidity.String()

	return s.repository.UpdateAddressLiquidityPool(alpFrom)
}

func (s *Service) getPoolChainFromTags(tags map[string]string) (map[uint64][]map[string]string, error) {
	poolsData := strings.Split(tags["tx.pools"], ",")
	data := make(map[uint64][]map[string]string)
	for _, p := range poolsData {
		pData := strings.Split(p, ":")
		poolId, err := strconv.ParseUint(pData[0], 10, 64)
		if err != nil {
			return nil, err
		}
		firstCoinData, err := s.getCoinVolumesFromTags(pData[1])
		if err != nil {
			return nil, err
		}
		secondCoinData, err := s.getCoinVolumesFromTags(pData[2])
		if err != nil {
			return nil, err
		}
		data[poolId] = []map[string]string{firstCoinData, secondCoinData}
	}
	return data, nil
}

func (s *Service) getCoinVolumesFromTags(str string) (map[string]string, error) {
	data := strings.Split(str, "-")
	result := make(map[string]string)
	result["coinId"] = data[0]
	result["volume"] = data[1]
	return result, nil
}

func NewService(repository *Repository, addressRepository *address.Repository, coinService *coin.Service,
	balanceService *balance.Service, logger *logrus.Entry) *Service {
	return &Service{
		repository:             repository,
		addressRepository:      addressRepository,
		coinService:            coinService,
		balanceService:         balanceService,
		logger:                 logger,
		jobUpdateLiquidityPool: make(chan *api_pb.TransactionResponse, 1),
	}
}
