package liquidity_pool

import (
	"encoding/json"
	"fmt"
	"github.com/MinterTeam/explorer-sdk/swap"
	"github.com/MinterTeam/minter-explorer-extender/v2/address"
	"github.com/MinterTeam/minter-explorer-extender/v2/balance"
	"github.com/MinterTeam/minter-explorer-extender/v2/coin"
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/MinterTeam/minter-explorer-tools/v4/helpers"
	"github.com/MinterTeam/minter-go-sdk/v2/api/grpc_client"
	"github.com/MinterTeam/minter-go-sdk/v2/transaction"
	"github.com/MinterTeam/node-grpc-gateway/api_pb"
	"github.com/go-pg/pg/v10"
	"github.com/sirupsen/logrus"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

func (s *Service) LiquidityPoolTradesSaveChannel() chan []*models.LiquidityPoolTrade {
	return s.liquidityPoolTradesSaveChannel
}

func (s *Service) SaveLiquidityPoolTradesWorker(data <-chan []*models.LiquidityPoolTrade) {
	for trades := range data {
		err := s.repository.SaveAllLiquidityPoolTrades(trades)
		if err != nil {
			s.logger.Error(err)
		}
	}
}

func (s *Service) LiquidityPoolTradesWorker(data <-chan []*models.Transaction) {
	chunkSize := 300
	for txs := range data {
		trades, err := s.getLiquidityPoolTrades(txs)
		if err != nil {
			s.logger.Error(err)
		}
		if len(trades) > 0 {
			chunksCount := int(math.Ceil(float64(len(trades)) / float64(chunkSize)))
			for i := 0; i < chunksCount; i++ {
				start := chunkSize * i
				end := start + chunkSize
				if end > len(trades) {
					end = len(trades)
				}
				s.LiquidityPoolTradesSaveChannel() <- trades[start:end]
			}
		}
		if err != nil {
			s.logger.Error(err)
		}
	}
}

func (s *Service) UpdateLiquidityPoolWorker(data <-chan *api_pb.TransactionResponse) {
	for tx := range data {
		var err error
		switch transaction.Type(tx.Type) {
		case transaction.TypeBuySwapPool,
			transaction.TypeSellSwapPool,
			transaction.TypeSellAllSwapPool:
			err = s.updateVolumesSwapPool(tx)
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

func (s *Service) LiquidityPoolTradesChannel() chan []*models.Transaction {
	return s.jobLiquidityPoolTrades
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

	txLiquidity, _ := big.NewInt(0).SetString(txTags["tx.liquidity"], 10)
	coinId, err := strconv.ParseUint(txTags["tx.pool_token_id"], 10, 64)
	if err != nil {
		return err
	}
	c, err := s.coinService.Storage.GetById(uint(coinId))
	if err != nil {
		return err
	}
	coinLiquidity, _ := big.NewInt(0).SetString(c.Volume, 10)
	coinLiquidity.Add(coinLiquidity, txLiquidity)
	c.Volume = coinLiquidity.String()
	err = s.coinService.Storage.Update(c)
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
		liquidity = big.NewInt(models.LockedLiquidityVolume)
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

	nodeLp, err := s.nodeApi.SwapPool(firstCoinId, secondCoinId)
	if err != nil {
		return nil, err
	} else {
		lp.Liquidity = nodeLp.Liquidity
		lp.FirstCoinVolume = nodeLp.Amount0
		lp.SecondCoinVolume = nodeLp.Amount1
	}

	lpList, err := s.repository.GetAll()
	if err != nil {
		return nil, err
	}

	if len(lpList) > 0 {
		liquidityBip := s.swapService.GetPoolLiquidity(lpList, *lp)
		lp.LiquidityBip = bigFloatToToPipString(liquidityBip)
	} else {
		lp.LiquidityBip = "0"
	}

	err = s.repository.UpdateLiquidityPool(lp)
	if err != nil {
		return nil, err
	}

	addressId, err := s.addressRepository.FindIdOrCreate(txFrom)
	if err != nil {
		return nil, err
	}

	nodeALP, err := s.nodeApi.SwapPoolProvider(firstCoinId, secondCoinId, fmt.Sprintf("Mx%s", txFrom))
	if err != nil {
		return nil, err
	}

	var alp *models.AddressLiquidityPool
	alp, err = s.repository.GetAddressLiquidityPool(addressId, lp.Id)
	if err != nil && err != pg.ErrNoRows {
		s.logger.Error(err)
	}
	if err != nil {
		alp = new(models.AddressLiquidityPool)
	}

	alp.AddressId = uint64(addressId)
	alp.Liquidity = nodeALP.Liquidity
	alp.LiquidityPoolId = lp.Id

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

	nodeLp, err := s.nodeApi.SwapPool(firstCoinId, secondCoinId)
	if err != nil {
		return err
	}

	lp.Liquidity = nodeLp.Liquidity
	lp.FirstCoinId = firstCoinId
	lp.SecondCoinId = secondCoinId
	lp.FirstCoinVolume = nodeLp.Amount0
	lp.SecondCoinVolume = nodeLp.Amount1

	lpList, err := s.repository.GetAll()
	if err != nil {
		return err
	}

	if len(lpList) > 0 {
		liquidityBip := s.swapService.GetPoolLiquidity(lpList, *lp)
		lp.LiquidityBip = bigFloatToToPipString(liquidityBip)
	} else {
		lp.LiquidityBip = "0"
	}

	err = s.repository.UpdateLiquidityPool(lp)
	if err != nil {
		return err
	}

	addressId, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(tx.From))
	if err != nil {
		return err
	}

	nodeALP, err := s.nodeApi.SwapPoolProvider(firstCoinId, secondCoinId, tx.From)
	if err != nil {
		return err
	}
	var alp *models.AddressLiquidityPool
	alp, err = s.repository.GetAddressLiquidityPool(addressId, lp.Id)
	if err != nil {
		s.logger.Error(err)
		alp = new(models.AddressLiquidityPool)
	}

	alp.AddressId = uint64(addressId)
	alp.LiquidityPoolId = lp.Id
	alp.Liquidity = nodeALP.Liquidity

	coinId, err := strconv.ParseUint(txTags["tx.pool_token_id"], 10, 64)
	if err != nil {
		return err
	}
	c, err := s.coinService.Storage.GetById(uint(coinId))
	if err != nil {
		return err
	}

	coinLiquidity, _ := big.NewInt(0).SetString(c.Volume, 10)
	coinLiquidity.Sub(coinLiquidity, txLiquidity)
	c.Volume = coinLiquidity.String()
	err = s.coinService.Storage.Update(c)
	if err != nil {
		return err
	}

	addressLiquidity, _ := big.NewInt(0).SetString(nodeALP.Liquidity, 10)
	if addressLiquidity.Cmp(big.NewInt(0)) == 0 {
		return s.repository.DeleteAddressLiquidityPool(addressId, lp.Id)
	} else {
		return s.repository.UpdateAddressLiquidityPool(alp)
	}
}

func (s *Service) updateVolumesSwapPool(tx *api_pb.TransactionResponse) error {
	var firstCoinId, secondCoinId uint64
	txTags := tx.GetTags()
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
			secondCoinId = coinId1
		} else {
			firstCoinId = coinId1
			secondCoinId = coinId0
		}

		lp, err := s.repository.getLiquidityPoolByCoinIds(firstCoinId, secondCoinId)
		if err != nil {
			return err
		}

		nodeLp, err := s.nodeApi.SwapPool(firstCoinId, secondCoinId)
		if err != nil {
			return err
		}

		lp.Liquidity = nodeLp.Liquidity
		lp.FirstCoinVolume = nodeLp.Amount0
		lp.SecondCoinVolume = nodeLp.Amount1

		lpList, err := s.repository.GetAll()
		if err != nil {
			return err
		}

		if len(lpList) > 0 {
			liquidityBip := s.swapService.GetPoolLiquidity(lpList, *lp)
			lp.LiquidityBip = bigFloatToToPipString(liquidityBip)
		} else {
			lp.LiquidityBip = "0"
		}

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

	nodeLp, err := s.nodeApi.SwapPool(0, tx.GasCoin.Id)
	if err != nil {
		return err
	}

	lp.FirstCoinVolume = nodeLp.Amount0
	lp.SecondCoinVolume = nodeLp.Amount1
	lp.Liquidity = nodeLp.Liquidity

	lpList, err := s.repository.GetAll()
	if err != nil {
		return err
	}

	if len(lpList) > 0 {
		liquidityBip := s.swapService.GetPoolLiquidity(lpList, *lp)
		lp.LiquidityBip = bigFloatToToPipString(liquidityBip)
	} else {
		lp.LiquidityBip = "0"
	}

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
	delta := big.NewInt(models.LockedLiquidityVolume)
	txValue.Sub(txValue, delta)

	addressFromLiquidity, _ := big.NewInt(0).SetString(alpFrom.Liquidity, 10)
	addressFromLiquidity.Sub(addressFromLiquidity, txValue)
	alpFrom.Liquidity = addressFromLiquidity.String()

	return s.repository.UpdateAddressLiquidityPool(alpFrom)
}

func (s *Service) getPoolChainFromTags(tags map[string]string) (map[uint64][]map[string]string, error) {
	var poolsData []models.TagLiquidityPool
	err := json.Unmarshal([]byte(tags["tx.pools"]), &poolsData)
	if err != nil {
		return nil, err
	}

	data := make(map[uint64][]map[string]string)
	for _, p := range poolsData {
		firstCoinData := make(map[string]string)
		firstCoinData["coinId"] = fmt.Sprintf("%d", p.CoinIn)
		firstCoinData["volume"] = p.ValueIn

		secondCoinData := make(map[string]string)
		secondCoinData["coinId"] = fmt.Sprintf("%d", p.CoinOut)
		secondCoinData["volume"] = p.ValueIn

		data[p.PoolID] = []map[string]string{firstCoinData, secondCoinData}
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

func (s *Service) getLiquidityPoolTrades(transactions []*models.Transaction) ([]*models.LiquidityPoolTrade, error) {
	var trades []*models.LiquidityPoolTrade
	for _, tx := range transactions {
		switch transaction.Type(tx.Type) {
		case transaction.TypeSellAllSwapPool,
			transaction.TypeSellSwapPool,
			transaction.TypeBuySwapPool:
			var poolsData []models.TagLiquidityPool
			err := json.Unmarshal([]byte(tx.Tags["tx.pools"]), &poolsData)
			if err != nil {
				return nil, err
			}
			trades = append(trades, s.getPoolTradesFromTagsData(tx.BlockID, tx.ID, poolsData)...)
		}
	}
	return trades, nil
}

func (s Service) getPoolTradesFromTagsData(blockId, transactionId uint64, poolsData []models.TagLiquidityPool) []*models.LiquidityPoolTrade {
	var trades []*models.LiquidityPoolTrade
	for _, p := range poolsData {
		var fcv, scv string
		if p.CoinIn < p.CoinOut {
			fcv = p.ValueIn
			scv = p.ValueOut
		} else {
			fcv = p.ValueOut
			scv = p.ValueIn
		}
		trades = append(trades, &models.LiquidityPoolTrade{
			BlockId:          blockId,
			LiquidityPoolId:  p.PoolID,
			TransactionId:    transactionId,
			FirstCoinVolume:  fcv,
			SecondCoinVolume: scv,
		})
	}
	return trades
}

func NewService(repository *Repository, addressRepository *address.Repository, coinService *coin.Service,
	balanceService *balance.Service, swapService *swap.Service, nodeApi *grpc_client.Client,
	logger *logrus.Entry) *Service {
	return &Service{
		repository:                     repository,
		addressRepository:              addressRepository,
		coinService:                    coinService,
		balanceService:                 balanceService,
		swapService:                    swapService,
		nodeApi:                        nodeApi,
		logger:                         logger,
		jobUpdateLiquidityPool:         make(chan *api_pb.TransactionResponse, 1),
		jobLiquidityPoolTrades:         make(chan []*models.Transaction, 1),
		liquidityPoolTradesSaveChannel: make(chan []*models.LiquidityPoolTrade, 10),
	}
}

type Service struct {
	repository                     *Repository
	addressRepository              *address.Repository
	coinService                    *coin.Service
	balanceService                 *balance.Service
	swapService                    *swap.Service
	logger                         *logrus.Entry
	nodeApi                        *grpc_client.Client
	jobUpdateLiquidityPool         chan *api_pb.TransactionResponse
	jobLiquidityPoolTrades         chan []*models.Transaction
	liquidityPoolTradesSaveChannel chan []*models.LiquidityPoolTrade
}

func bigFloatToToPipString(f *big.Float) string {
	s := f.String()
	num := strings.Split(f.String(), ".")
	s = num[0]
	count := 0
	if len(num) > 1 {
		s += num[1]
		count = len(num[1])
	}
	for i := 0; i < (18 - count); i++ {
		s += "0"
	}
	return s
}
