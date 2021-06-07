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
	"time"
)

func (s *Service) CreateSnapshot(height uint64, date time.Time) error {
	list, err := s.Storage.GetAll()
	if err != nil && err != pg.ErrNoRows {
		return err
	}
	if err != nil && err == pg.ErrNoRows {
		return nil
	}
	var snap []models.LiquidityPoolSnapshot
	for _, p := range list {
		snap = append(snap, models.LiquidityPoolSnapshot{
			BlockId:          height,
			LiquidityPoolId:  p.Id,
			FirstCoinVolume:  p.FirstCoinVolume,
			SecondCoinVolume: p.SecondCoinVolume,
			Liquidity:        p.Liquidity,
			LiquidityBip:     p.LiquidityBip,
			CreatedAt:        date,
		})
	}
	return s.Storage.SaveLiquidityPoolSnapshots(snap)
}

func (s *Service) LiquidityPoolTradesSaveChannel() chan []*models.LiquidityPoolTrade {
	return s.liquidityPoolTradesSaveChannel
}

func (s *Service) SaveLiquidityPoolTradesWorker(data <-chan []*models.LiquidityPoolTrade) {
	for trades := range data {
		err := s.Storage.SaveAllLiquidityPoolTrades(trades)
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
			err = s.CreateLiquidityPool(tx)
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

func (s *Service) CreateLiquidityPool(tx *api_pb.TransactionResponse) error {
	txData := new(api_pb.CreateSwapPoolData)
	if err := tx.GetData().UnmarshalTo(txData); err != nil {
		return err
	}

	txTags := tx.GetTags()

	var (
		firstCoinId, secondCoinId uint64
	)

	if txData.Coin0.Id < txData.Coin1.Id {
		firstCoinId = txData.Coin0.Id
		secondCoinId = txData.Coin1.Id
	} else {
		firstCoinId = txData.Coin1.Id
		secondCoinId = txData.Coin0.Id
	}

	_, err := s.coinService.CreatePoolToken(tx)
	if err != nil {
		return err
	}

	_, err = s.addToPool(tx.Height, firstCoinId, secondCoinId, helpers.RemovePrefix(tx.From), txTags)
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
		firstCoinId, secondCoinId uint64
	)

	if txData.Coin0.Id < txData.Coin1.Id {
		firstCoinId = txData.Coin0.Id
		secondCoinId = txData.Coin1.Id
	} else {
		firstCoinId = txData.Coin1.Id
		secondCoinId = txData.Coin0.Id
	}

	_, err := s.addToPool(tx.Height, firstCoinId, secondCoinId, helpers.RemovePrefix(tx.From), txTags)
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

func (s *Service) addToPool(height, firstCoinId, secondCoinId uint64, txFrom string,
	txTags map[string]string) (*models.LiquidityPool, error) {

	lp, err := s.Storage.getLiquidityPoolByCoinIds(firstCoinId, secondCoinId)
	if err != nil && err != pg.ErrNoRows {
		return nil, err
	} else if lp == nil {
		lp = new(models.LiquidityPool)
	}

	txPoolToken := strings.Split(txTags["tx.pool_token"], "-")
	lpId, err := strconv.ParseUint(txPoolToken[1], 10, 64)
	if err != nil {
		return nil, err
	}

	coinId, err := strconv.ParseUint(txTags["tx.pool_token_id"], 10, 64)
	if err != nil {
		return nil, err
	}

	var nodeLp *api_pb.SwapPoolResponse
	if s.chasingMode {
		nodeLp, err = s.nodeApi.SwapPool(firstCoinId, secondCoinId, height)
	} else {
		nodeLp, err = s.nodeApi.SwapPool(firstCoinId, secondCoinId)
	}

	if err != nil {
		return nil, err
	} else {
		lp.Id = lpId
		lp.TokenId = coinId
		lp.FirstCoinId = firstCoinId
		lp.SecondCoinId = secondCoinId
		lp.Liquidity = nodeLp.Liquidity
		lp.FirstCoinVolume = nodeLp.Amount0
		lp.SecondCoinVolume = nodeLp.Amount1
		lp.UpdatedAtBlockId = height
	}

	lpList, err := s.Storage.GetAll()
	if err != nil {
		return nil, err
	}

	if len(lpList) > 0 {
		liquidityBip := s.swapService.GetPoolLiquidity(lpList, *lp)
		lp.LiquidityBip = bigFloatToPipString(liquidityBip)
	} else {
		lp.LiquidityBip = "0"
	}

	err = s.Storage.UpdateLiquidityPool(lp)
	if err != nil {
		return nil, err
	}

	addressId, err := s.addressRepository.FindIdOrCreate(txFrom)
	if err != nil {
		return nil, err
	}

	var nodeALP *api_pb.SwapPoolResponse
	if s.chasingMode {
		nodeALP, err = s.nodeApi.SwapPoolProvider(firstCoinId, secondCoinId, fmt.Sprintf("Mx%s", txFrom), height)
	} else {
		nodeALP, err = s.nodeApi.SwapPoolProvider(firstCoinId, secondCoinId, fmt.Sprintf("Mx%s", txFrom))
	}
	if err != nil {
		return nil, err
	}

	var alp *models.AddressLiquidityPool
	alp, err = s.Storage.GetAddressLiquidityPool(addressId, lp.Id)
	if err != nil && err != pg.ErrNoRows {
		s.logger.Error(err)
	}
	if err != nil {
		alp = new(models.AddressLiquidityPool)
	}

	alp.AddressId = uint64(addressId)
	alp.Liquidity = nodeALP.Liquidity
	alp.FirstCoinVolume = nodeALP.Amount0
	alp.SecondCoinVolume = nodeALP.Amount1
	alp.LiquidityPoolId = lp.Id

	if nodeALP.Liquidity == "0" {
		err = s.Storage.DeleteAddressLiquidityPool(addressId, lp.Id)
	} else {
		err = s.Storage.UpdateAddressLiquidityPool(alp)
	}

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
		firstCoinId, secondCoinId uint64
	)

	if txData.Coin0.Id < txData.Coin1.Id {
		firstCoinId = txData.Coin0.Id
		secondCoinId = txData.Coin1.Id
	} else {
		firstCoinId = txData.Coin1.Id
		secondCoinId = txData.Coin0.Id
	}

	lp, err := s.Storage.getLiquidityPoolByCoinIds(firstCoinId, secondCoinId)
	if err != nil && err != pg.ErrNoRows {
		return err
	}

	var nodeLp *api_pb.SwapPoolResponse
	if s.chasingMode {
		nodeLp, err = s.nodeApi.SwapPool(firstCoinId, secondCoinId, tx.Height)
	} else {
		nodeLp, err = s.nodeApi.SwapPool(firstCoinId, secondCoinId)
	}
	if err != nil {
		return err
	}

	lp.Liquidity = nodeLp.Liquidity
	lp.FirstCoinId = firstCoinId
	lp.SecondCoinId = secondCoinId
	lp.FirstCoinVolume = nodeLp.Amount0
	lp.SecondCoinVolume = nodeLp.Amount1
	lp.UpdatedAtBlockId = tx.Height

	lpList, err := s.Storage.GetAll()
	if err != nil {
		return err
	}

	if len(lpList) > 0 {
		liquidityBip := s.swapService.GetPoolLiquidity(lpList, *lp)
		lp.LiquidityBip = bigFloatToPipString(liquidityBip)
	} else {
		lp.LiquidityBip = "0"
	}

	err = s.Storage.UpdateLiquidityPool(lp)
	if err != nil {
		return err
	}

	addressId, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(tx.From))
	if err != nil {
		return err
	}

	var nodeALP *api_pb.SwapPoolResponse
	if s.chasingMode {
		nodeALP, err = s.nodeApi.SwapPoolProvider(firstCoinId, secondCoinId, tx.From, tx.Height)
	} else {
		nodeALP, err = s.nodeApi.SwapPoolProvider(firstCoinId, secondCoinId, tx.From)
	}
	if err != nil {
		return err
	}

	var alp *models.AddressLiquidityPool
	alp, err = s.Storage.GetAddressLiquidityPool(addressId, lp.Id)
	if err != nil && err != pg.ErrNoRows {
		return err
	}
	if err != nil {
		alp = new(models.AddressLiquidityPool)
	}

	alp.AddressId = uint64(addressId)
	alp.LiquidityPoolId = lp.Id
	alp.Liquidity = nodeALP.Liquidity
	alp.FirstCoinVolume = nodeALP.Amount0
	alp.SecondCoinVolume = nodeALP.Amount1

	coinId, err := strconv.ParseUint(txTags["tx.pool_token_id"], 10, 64)
	if err != nil {
		return err
	}
	c, err := s.coinService.Storage.GetById(uint(coinId))
	if err != nil {
		return err
	}

	liquidity := big.NewInt(0)
	if lp.Liquidity != "" {
		liquidity, _ = big.NewInt(0).SetString(lp.Liquidity, 10)
	}
	txLiquidity, _ := big.NewInt(0).SetString(txData.Liquidity, 10)
	liquidity.Sub(liquidity, txLiquidity)

	coinLiquidity, _ := big.NewInt(0).SetString(c.Volume, 10)
	coinLiquidity.Sub(coinLiquidity, txLiquidity)
	c.Volume = coinLiquidity.String()
	err = s.coinService.Storage.Update(c)
	if err != nil {
		return err
	}

	if nodeALP.Liquidity == "0" {
		return s.Storage.DeleteAddressLiquidityPool(addressId, lp.Id)
	} else {
		return s.Storage.UpdateAddressLiquidityPool(alp)
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

		lp, err := s.Storage.getLiquidityPoolByCoinIds(firstCoinId, secondCoinId)
		if err != nil {
			return err
		}

		var nodeLp *api_pb.SwapPoolResponse
		if s.chasingMode {
			nodeLp, err = s.nodeApi.SwapPool(firstCoinId, secondCoinId, tx.Height)
		} else {
			nodeLp, err = s.nodeApi.SwapPool(firstCoinId, secondCoinId)
		}
		if err != nil {
			return err
		}

		lp.Liquidity = nodeLp.Liquidity
		lp.FirstCoinVolume = nodeLp.Amount0
		lp.SecondCoinVolume = nodeLp.Amount1
		lp.UpdatedAtBlockId = tx.Height

		lpList, err := s.Storage.GetAll()
		if err != nil {
			return err
		}

		if len(lpList) > 0 {
			liquidityBip := s.swapService.GetPoolLiquidity(lpList, *lp)
			lp.LiquidityBip = bigFloatToPipString(liquidityBip)
		} else {
			lp.LiquidityBip = "0"
		}

		err = s.Storage.UpdateLiquidityPool(lp)
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

	lp, err := s.Storage.getLiquidityPoolByCoinIds(0, tx.GasCoin.Id)
	if err != nil {
		return err
	}

	var nodeLp *api_pb.SwapPoolResponse
	if s.chasingMode {
		nodeLp, err = s.nodeApi.SwapPool(0, tx.GasCoin.Id, tx.Height)
	} else {
		nodeLp, err = s.nodeApi.SwapPool(0, tx.GasCoin.Id)
	}
	if err != nil {
		return err
	}

	lp.FirstCoinVolume = nodeLp.Amount0
	lp.SecondCoinVolume = nodeLp.Amount1
	lp.Liquidity = nodeLp.Liquidity
	lp.UpdatedAtBlockId = tx.Height

	lpList, err := s.Storage.GetAll()
	if err != nil {
		return err
	}

	if len(lpList) > 0 {
		liquidityBip := s.swapService.GetPoolLiquidity(lpList, *lp)
		lp.LiquidityBip = bigFloatToPipString(liquidityBip)
	} else {
		lp.LiquidityBip = "0"
	}

	return s.Storage.UpdateLiquidityPool(lp)
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
		return s.Storage.getLiquidityPoolByCoinIds(firstCoinId, secondCoinId)
	} else {
		return s.Storage.getLiquidityPoolByCoinIds(secondCoinId, firstCoinId)
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
	return s.Storage.GetAllByIds(idList)
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

		var re = regexp.MustCompile(`(?mi)lp-\d+`)
		if re.MatchString(txData.Coin.Symbol) {
			err = s.updateAddressPoolVolumesByTxData(fromAddressId, tx.From, tx.Height, txData)
		}

	case transaction.TypeMultisend:
		txData := new(api_pb.MultiSendData)
		if err = tx.Data.UnmarshalTo(txData); err != nil {
			return err
		}
		for _, data := range txData.List {
			var re = regexp.MustCompile(`(?mi)lp-\d+`)
			if re.MatchString(data.Coin.Symbol) {
				err = s.updateAddressPoolVolumesByTxData(fromAddressId, tx.From, tx.Height, data)
				if err != nil {
					return err
				}
			}
		}
	}

	return err
}

func (s *Service) updateAddressPoolVolumesByTxData(fromAddressId uint, from string, height uint64, txData *api_pb.SendData) error {
	toAddressId, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(txData.To))
	if err != nil {
		return err
	}

	lp, err := s.Storage.getLiquidityPoolByTokenId(txData.Coin.Id)
	if err != nil {
		return err
	}

	var nodeALPFrom *api_pb.SwapPoolResponse
	if s.chasingMode {
		nodeALPFrom, err = s.nodeApi.SwapPoolProvider(lp.FirstCoinId, lp.SecondCoinId, from, height)
	} else {
		nodeALPFrom, err = s.nodeApi.SwapPoolProvider(lp.FirstCoinId, lp.SecondCoinId, from)
	}
	if err != nil {
		return err
	}

	alpFrom := &models.AddressLiquidityPool{
		LiquidityPoolId:  lp.Id,
		AddressId:        uint64(fromAddressId),
		FirstCoinVolume:  nodeALPFrom.Amount0,
		SecondCoinVolume: nodeALPFrom.Amount1,
		Liquidity:        nodeALPFrom.Liquidity,
	}

	var nodeALPTo *api_pb.SwapPoolResponse
	if s.chasingMode {
		nodeALPTo, err = s.nodeApi.SwapPoolProvider(lp.FirstCoinId, lp.SecondCoinId, txData.To, height)
	} else {
		nodeALPTo, err = s.nodeApi.SwapPoolProvider(lp.FirstCoinId, lp.SecondCoinId, txData.To)
	}
	if err != nil {
		return err
	}
	alpTo := &models.AddressLiquidityPool{
		LiquidityPoolId:  lp.Id,
		AddressId:        uint64(toAddressId),
		FirstCoinVolume:  nodeALPTo.Amount0,
		SecondCoinVolume: nodeALPTo.Amount1,
		Liquidity:        nodeALPTo.Liquidity,
	}

	return s.Storage.UpdateAllLiquidityPool([]*models.AddressLiquidityPool{alpFrom, alpTo})
}

func (s *Service) updateAddressPoolVolumesWhenCreate(fromAddressId uint, lpId uint64, value string) error {
	alpFrom, err := s.Storage.GetAddressLiquidityPool(fromAddressId, lpId)
	if err != nil {
		return err
	}
	txValue, _ := big.NewInt(0).SetString(value, 10)
	delta := big.NewInt(models.LockedLiquidityVolume)
	txValue.Sub(txValue, delta)

	addressFromLiquidity, _ := big.NewInt(0).SetString(alpFrom.Liquidity, 10)
	addressFromLiquidity.Sub(addressFromLiquidity, txValue)
	alpFrom.Liquidity = addressFromLiquidity.String()

	return s.Storage.UpdateAddressLiquidityPool(alpFrom)
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

func (s *Service) SetChasingMode(chasingMode bool) {
	s.chasingMode = chasingMode
}

func NewService(repository *Repository, addressRepository *address.Repository, coinService *coin.Service,
	balanceService *balance.Service, swapService *swap.Service, nodeApi *grpc_client.Client,
	logger *logrus.Entry) *Service {
	return &Service{
		Storage:                        repository,
		addressRepository:              addressRepository,
		coinService:                    coinService,
		balanceService:                 balanceService,
		swapService:                    swapService,
		nodeApi:                        nodeApi,
		logger:                         logger,
		chasingMode:                    false,
		jobUpdateLiquidityPool:         make(chan *api_pb.TransactionResponse, 1),
		jobLiquidityPoolTrades:         make(chan []*models.Transaction, 1),
		liquidityPoolTradesSaveChannel: make(chan []*models.LiquidityPoolTrade, 10),
	}
}

type Service struct {
	Storage                        *Repository
	addressRepository              *address.Repository
	coinService                    *coin.Service
	balanceService                 *balance.Service
	swapService                    *swap.Service
	logger                         *logrus.Entry
	nodeApi                        *grpc_client.Client
	jobUpdateLiquidityPool         chan *api_pb.TransactionResponse
	jobLiquidityPoolTrades         chan []*models.Transaction
	liquidityPoolTradesSaveChannel chan []*models.LiquidityPoolTrade
	chasingMode                    bool
}

func bigFloatToPipString(f *big.Float) string {
	pip, _ := new(big.Float).Mul(big.NewFloat(1e18), f).Int(nil)
	return pip.String()
}
