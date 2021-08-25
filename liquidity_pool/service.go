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
	"sync"
	"time"
)

func (s *Service) AddressLiquidityPoolWorker() {
	var err error
	wg := sync.WaitGroup{}
	for {
		txs := <-s.updateAddressPoolChannel
		wg.Add(len(txs))
		for _, tx := range txs {
			go func(tx *api_pb.TransactionResponse) {
				if tx.Log == "" {
					switch transaction.Type(tx.Type) {
					case transaction.TypeRemoveLiquidity,
						transaction.TypeAddLiquidity:
						txFrom := helpers.RemovePrefix(tx.From)
						txTags := tx.GetTags()
						poolId, err := strconv.ParseUint(txTags["tx.pool_id"], 10, 64)
						if err != nil {
							s.logger.Error(err)
						}

						pair := strings.Split(txTags["tx.pair_ids"], "-")
						firstCoinId, err := strconv.ParseUint(pair[0], 10, 64)
						if err != nil {
							s.logger.Error(err)
						}
						secondCoinId, err := strconv.ParseUint(pair[1], 10, 64)
						if err != nil {
							s.logger.Error(err)
						}

						var nodeALP *api_pb.SwapPoolResponse
						if s.chasingMode {
							nodeALP, err = s.nodeApi.SwapPoolProvider(firstCoinId, secondCoinId, fmt.Sprintf("Mx%s", txFrom), tx.Height)
						} else {
							nodeALP, err = s.nodeApi.SwapPoolProvider(firstCoinId, secondCoinId, fmt.Sprintf("Mx%s", txFrom))
						}
						if err != nil {
							s.logger.Error(err)
						}

						addressId, err := s.addressRepository.FindIdOrCreate(txFrom)
						if err != nil {
							s.logger.Error(err)
						}

						var alp *models.AddressLiquidityPool
						alp, err = s.Storage.GetAddressLiquidityPool(addressId, poolId)
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
						alp.LiquidityPoolId = poolId

						if nodeALP.Liquidity == "0" {
							err = s.Storage.DeleteAddressLiquidityPool(addressId, poolId)
						} else {
							err = s.Storage.UpdateAddressLiquidityPool(alp)
						}
						if err != nil {
							s.logger.Error(err)
						}
					case transaction.TypeSend,
						transaction.TypeMultisend,
						transaction.TypeBuySwapPool,
						transaction.TypeSellSwapPool,
						transaction.TypeSellAllSwapPool:
						err = s.updateAddressPoolVolumes(tx)
						if err != nil {
							s.logger.Error(err)
						}
					}
				}
				wg.Done()
			}(tx)
		}
		wg.Wait()
		err = s.Storage.RemoveEmptyAddresses()
		if err != nil {
			s.logger.Error(err)
		}
	}
}

func (s *Service) LiquidityPoolWorker(data <-chan *api_pb.BlockResponse) {
	for b := range data {
		var lpList []uint64
		for _, tx := range b.Transactions {
			if tx.Log == "" {
				tags := tx.GetTags()
				if tags["tx.commission_conversion"] == "pool" {
					lp, err := s.Storage.getLiquidityPoolByCoinIds(0, tx.GasCoin.Id)
					if err != nil {
						s.logger.WithFields(logrus.Fields{
							"block": b.Height,
							"coin0": 0,
							"coin1": tx.GasCoin.Id,
							"tx":    tx.RawTx,
						}).Error(err)
						continue
					}
					lpList = append(lpList, lp.Id)
				}

				switch transaction.Type(tx.Type) {
				case transaction.TypeRemoveLiquidity,
					transaction.TypeAddLiquidity,
					transaction.TypeBuySwapPool,
					transaction.TypeSellSwapPool,
					transaction.TypeSellAllSwapPool:
					list, err := s.GetLiquidityPoolsIdFromTx(tx)
					if err != nil {
						s.logger.WithFields(logrus.Fields{
							"block": b.Height,
							"tx":    tx.RawTx,
						}).Error(err)
						continue
					}
					lpList = append(lpList, list...)
				case transaction.TypeSend:
					txData := new(api_pb.SendData)
					if err := tx.Data.UnmarshalTo(txData); err != nil {
						s.logger.WithFields(logrus.Fields{
							"coinId": txData.Coin.Id,
							"block":  b.Height,
							"tx":     tx.RawTx,
						}).Error(err)
						continue
					}
					var re = regexp.MustCompile(`(?mi)lp-\d+`)
					if re.MatchString(txData.Coin.Symbol) {
						lp, err := s.Storage.getLiquidityPoolByTokenId(txData.Coin.Id)
						if err != nil {
							s.logger.WithFields(logrus.Fields{
								"coinId": txData.Coin.Id,
								"block":  b.Height,
								"tx":     tx.RawTx,
							}).Error(err)
							continue
						}
						lpList = append(lpList, lp.Id)
					}

				case transaction.TypeMultisend:
					txData := new(api_pb.MultiSendData)
					if err := tx.Data.UnmarshalTo(txData); err != nil {
						s.logger.Error(err)
						continue
					}
					for _, data := range txData.List {
						var re = regexp.MustCompile(`(?mi)lp-\d+`)
						if re.MatchString(data.Coin.Symbol) {
							lp, err := s.Storage.getLiquidityPoolByTokenId(data.Coin.Id)
							if err != nil {
								s.logger.WithFields(logrus.Fields{
									"block": b.Height,
									"tx":    tx.RawTx,
								}).Error(err)
								continue
							}
							lpList = append(lpList, lp.Id)
						}
					}
				}
			}
		}

		if len(lpList) < 1 {
			continue
		}

		uniqId := make(map[uint64]struct{})
		for _, id := range lpList {
			uniqId[id] = struct{}{}
		}

		var lpsList []uint64
		for id := range uniqId {
			lpsList = append(lpsList, id)
		}

		var lps []models.LiquidityPool
		var err error

		lps, err = s.Storage.GetAllByIds(lpsList)
		if err != nil {
			s.logger.Error(err)
			continue
		}

		coinsForUpdate := make(map[uint64]struct{})
		for _, lp := range lps {
			coinsForUpdate[lp.TokenId] = struct{}{}
		}
		s.coinService.GetUpdateCoinsFromCoinsMapJobChannel() <- coinsForUpdate

		wg := sync.WaitGroup{}
		wg.Add(len(lps))
		for _, lp := range lps {
			go func(lp models.LiquidityPool) {
				err := s.updateLiquidityPool(b.Height, lp)
				if err != nil {
					s.logger.Error(err)
				}
				wg.Done()
			}(lp)
		}
		wg.Wait()

		lps, err = s.Storage.GetAllByIds(lpsList)
		if err != nil {
			s.logger.Error(err)
			continue
		}
		s.updatePoolsBipLiquidity(lps)
		s.updateAddressPoolChannel <- b.Transactions
	}
}

func (s *Service) GetLiquidityPoolsIdFromTx(tx *api_pb.TransactionResponse) ([]uint64, error) {
	var err error
	var ids []uint64
	var re = regexp.MustCompile(`(?mi)lp-\d+`)
	switch transaction.Type(tx.Type) {
	case transaction.TypeBuySwapPool:
		txData := new(api_pb.BuySwapPoolData)
		if err := tx.Data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		for _, c := range txData.Coins {
			if re.MatchString(c.Symbol) {
				p, err := s.Storage.getLiquidityPoolByTokenId(c.Id)
				if err != nil {
					return nil, err
				}
				ids = append(ids, p.Id)
			}
		}
		txTags := tx.GetTags()
		list, err := s.getPoolChainFromTags(txTags)
		if err != nil {
			return nil, err
		}
		for id := range list {
			ids = append(ids, id)
		}
	case transaction.TypeSellSwapPool:
		txData := new(api_pb.SellSwapPoolData)
		if err := tx.Data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		for _, c := range txData.Coins {
			if re.MatchString(c.Symbol) {
				p, err := s.Storage.getLiquidityPoolByTokenId(c.Id)
				if err != nil {
					return nil, err
				}
				ids = append(ids, p.Id)
			}
		}
		txTags := tx.GetTags()
		list, err := s.getPoolChainFromTags(txTags)
		if err != nil {
			return nil, err
		}
		for id := range list {
			ids = append(ids, id)
		}
	case transaction.TypeSellAllSwapPool:
		txData := new(api_pb.SellAllSwapPoolData)
		if err := tx.Data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		for _, c := range txData.Coins {
			if re.MatchString(c.Symbol) {
				p, err := s.Storage.getLiquidityPoolByTokenId(c.Id)
				if err != nil {
					return nil, err
				}
				ids = append(ids, p.Id)
			}
		}
		txTags := tx.GetTags()
		list, err := s.getPoolChainFromTags(txTags)
		if err != nil {
			return nil, err
		}
		for id := range list {
			ids = append(ids, id)
		}
	case transaction.TypeCreateSwapPool:
		txData := new(api_pb.CreateSwapPoolData)
		if err := tx.Data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		if re.MatchString(txData.Coin0.Symbol) {
			p, err := s.Storage.getLiquidityPoolByTokenId(txData.Coin0.Id)
			if err != nil {
				return nil, err
			}
			ids = append(ids, p.Id)
		}
		if re.MatchString(txData.Coin1.Symbol) {
			p, err := s.Storage.getLiquidityPoolByTokenId(txData.Coin1.Id)
			if err != nil {
				return nil, err
			}
			ids = append(ids, p.Id)
		}
		txTags := tx.GetTags()
		id, err := strconv.ParseUint(txTags["tx.pool_id"], 10, 64)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	case transaction.TypeAddLiquidity:
		txData := new(api_pb.AddLiquidityData)
		if err := tx.Data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		if re.MatchString(txData.Coin0.Symbol) {
			p, err := s.Storage.getLiquidityPoolByTokenId(txData.Coin0.Id)
			if err != nil {
				return nil, err
			}
			ids = append(ids, p.Id)
		}
		if re.MatchString(txData.Coin1.Symbol) {
			p, err := s.Storage.getLiquidityPoolByTokenId(txData.Coin1.Id)
			if err != nil {
				return nil, err
			}
			ids = append(ids, p.Id)
		}
		txTags := tx.GetTags()
		id, err := strconv.ParseUint(txTags["tx.pool_id"], 10, 64)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	case transaction.TypeRemoveLiquidity:
		txData := new(api_pb.RemoveLiquidityData)
		if err := tx.Data.UnmarshalTo(txData); err != nil {
			return nil, err
		}
		if re.MatchString(txData.Coin0.Symbol) {
			p, err := s.Storage.getLiquidityPoolByTokenId(txData.Coin0.Id)
			if err != nil {
				return nil, err
			}
			ids = append(ids, p.Id)
		}
		if re.MatchString(txData.Coin1.Symbol) {
			p, err := s.Storage.getLiquidityPoolByTokenId(txData.Coin1.Id)
			if err != nil {
				return nil, err
			}
			ids = append(ids, p.Id)
		}
		txTags := tx.GetTags()
		id, err := strconv.ParseUint(txTags["tx.pool_id"], 10, 64)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, err
}

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

func (s *Service) LiquidityPoolTradesChannel() chan []*models.Transaction {
	return s.jobLiquidityPoolTrades
}

func (s *Service) LiquidityPoolTradesSaveChannel() chan []*models.LiquidityPoolTrade {
	return s.liquidityPoolTradesSaveChannel
}

func (s *Service) SaveLiquidityPoolTradesWorker(data <-chan []*models.LiquidityPoolTrade) {
	for trades := range data {
		var err error
		if len(trades) > 0 {
			err = s.Storage.SaveAllLiquidityPoolTrades(trades)
		}
		if err != nil {
			if len(trades) > 0 {
				lf := logrus.Fields{}
				for index, i := range trades {
					lf[fmt.Sprintf("%d", index)] = fmt.Sprintf("BlockId: %d, LpId: %d, TxId:  %d", i.BlockId, i.LiquidityPoolId, i.TransactionId)
				}
				s.logger.WithFields(lf).Error(err)
			} else {
				s.logger.Error(err)
			}
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
		s.logger.Info(fmt.Sprintf("Pool %d Liquidity Bip: %s", lp.Id, liquidityBip.Text('f', 18)))
		lp.LiquidityBip = s.bigFloatToPipString(liquidityBip)
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

	err = s.Storage.UpdateAddressLiquidityPool(alp)

	if err != nil {
		return nil, err
	}

	return lp, err
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

	var mapForUpdate = make(map[uint64][]string)

	switch transaction.Type(tx.Type) {
	case transaction.TypeSend:
		txData := new(api_pb.SendData)
		if err = tx.Data.UnmarshalTo(txData); err != nil {
			return err
		}
		var re = regexp.MustCompile(`(?mi)lp-\d+`)
		if re.MatchString(txData.Coin.Symbol) {
			err = s.updateAddressPoolVolumesBySendData(fromAddressId, tx.From, tx.Height, txData)
		}

	case transaction.TypeMultisend:
		txData := new(api_pb.MultiSendData)
		if err = tx.Data.UnmarshalTo(txData); err != nil {
			return err
		}
		for _, data := range txData.List {
			var re = regexp.MustCompile(`(?mi)lp-\d+`)
			if re.MatchString(data.Coin.Symbol) {
				err = s.updateAddressPoolVolumesBySendData(fromAddressId, tx.From, tx.Height, data)
				if err != nil {
					return err
				}
			}
		}

	case transaction.TypeBuySwapPool:
		txData := new(api_pb.BuySwapPoolData)
		if err = tx.Data.UnmarshalTo(txData); err != nil {
			return err
		}
		var re = regexp.MustCompile(`(?mi)lp-\d+`)
		for _, c := range txData.Coins {
			if re.MatchString(c.Symbol) {
				err := s.updateAddressPoolVolumesByBuySellData(fromAddressId, tx.From, tx.Height, c.Id)
				if err != nil {
					return err
				}
			}
		}
		mapForUpdate, err = s.getAddressesForVolUpdateFromTag(tx)
		if err != nil {
			return err
		}

	case transaction.TypeSellSwapPool:
		txData := new(api_pb.SellSwapPoolData)
		if err = tx.Data.UnmarshalTo(txData); err != nil {
			return err
		}
		var re = regexp.MustCompile(`(?mi)lp-\d+`)
		for _, c := range txData.Coins {
			if re.MatchString(c.Symbol) {
				err := s.updateAddressPoolVolumesByBuySellData(fromAddressId, tx.From, tx.Height, c.Id)
				if err != nil {
					return err
				}
			}
		}
		mapForUpdate, err = s.getAddressesForVolUpdateFromTag(tx)
		if err != nil {
			return err
		}

	case transaction.TypeSellAllSwapPool:
		txData := new(api_pb.SellAllSwapPoolData)
		if err = tx.Data.UnmarshalTo(txData); err != nil {
			return err
		}
		var re = regexp.MustCompile(`(?mi)lp-\d+`)
		for _, c := range txData.Coins {
			if re.MatchString(c.Symbol) {
				err := s.updateAddressPoolVolumesByBuySellData(fromAddressId, tx.From, tx.Height, c.Id)
				if err != nil {
					return err
				}
			}
		}
		mapForUpdate, err = s.getAddressesForVolUpdateFromTag(tx)
		if err != nil {
			return err
		}
	}

	if len(mapForUpdate) > 0 {
		err = s.updateAddressesVolumesFromMap(mapForUpdate, tx.Height)
	}

	return err
}

func (s *Service) updateAddressesVolumesFromMap(addresses map[uint64][]string, height uint64) error {
	wg := sync.WaitGroup{}
	var alpMap sync.Map
	for poolId, addressList := range addresses {
		lp, err := s.Storage.getLiquidityPoolById(poolId)
		if err != nil {
			return err
		}

		wg.Add(len(addressList))
		go func(addresses []string, poolId, height uint64) {
			for _, a := range addresses {
				addressId, err := s.addressRepository.FindIdOrCreate(a)
				if err != nil {
					s.logger.Error(err)
					wg.Done()
					return
				}

				var nodeALP *api_pb.SwapPoolResponse
				if s.chasingMode {
					nodeALP, err = s.nodeApi.SwapPoolProvider(lp.FirstCoinId, lp.SecondCoinId, fmt.Sprintf("Mx%s", a), height)
				} else {
					nodeALP, err = s.nodeApi.SwapPoolProvider(lp.FirstCoinId, lp.SecondCoinId, fmt.Sprintf("Mx%s", a))
				}
				if err != nil {
					s.logger.Error(err)
					wg.Done()
					return
				}
				alpMap.Store(fmt.Sprintf("%d-%d", poolId, addressId), &models.AddressLiquidityPool{
					LiquidityPoolId:  poolId,
					AddressId:        uint64(addressId),
					FirstCoinVolume:  nodeALP.Amount0,
					SecondCoinVolume: nodeALP.Amount1,
					Liquidity:        nodeALP.Liquidity,
				})
				wg.Done()
			}
		}(addressList, poolId, height)
	}

	var forUpdate []*models.AddressLiquidityPool
	alpMap.Range(func(k, v interface{}) bool {
		forUpdate = append(forUpdate, v.(*models.AddressLiquidityPool))
		return true
	})

	if len(forUpdate) > 0 {
		return s.Storage.UpdateAllLiquidityPool(forUpdate)
	}

	return nil
}

func (s *Service) updateAddressPoolVolumesByBuySellData(fromAddressId uint, from string, height, lpTokenId uint64) error {

	lp, err := s.Storage.getLiquidityPoolByTokenId(lpTokenId)
	if err != nil {
		return err
	}

	var nodeALP *api_pb.SwapPoolResponse
	if s.chasingMode {
		nodeALP, err = s.nodeApi.SwapPoolProvider(lp.FirstCoinId, lp.SecondCoinId, from, height)
	} else {
		nodeALP, err = s.nodeApi.SwapPoolProvider(lp.FirstCoinId, lp.SecondCoinId, from)
	}
	if err != nil {
		return err
	}

	alp := &models.AddressLiquidityPool{
		LiquidityPoolId:  lp.Id,
		AddressId:        uint64(fromAddressId),
		FirstCoinVolume:  nodeALP.Amount0,
		SecondCoinVolume: nodeALP.Amount1,
		Liquidity:        nodeALP.Liquidity,
	}

	return s.Storage.UpdateAllLiquidityPool([]*models.AddressLiquidityPool{alp})
}

func (s *Service) updateAddressPoolVolumesBySendData(fromAddressId uint, from string, height uint64, txData *api_pb.SendData) error {
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
			trades = append(trades, s.getPoolTradesFromTagsData(tx, poolsData)...)
		}
	}
	return trades, nil
}

func (s Service) getPoolTradesFromTagsData(tx *models.Transaction, poolsData []models.TagLiquidityPool) []*models.LiquidityPoolTrade {
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
			BlockId:          tx.BlockID,
			LiquidityPoolId:  p.PoolID,
			TransactionId:    tx.ID,
			FirstCoinVolume:  fcv,
			SecondCoinVolume: scv,
			CreatedAt:        tx.CreatedAt,
		})
	}
	return trades
}

func (s *Service) SetChasingMode(chasingMode bool) {
	s.chasingMode = chasingMode
}

func (s *Service) getAddressesForVolUpdateFromTag(tx *api_pb.TransactionResponse) (map[uint64][]string, error) {
	tags := tx.GetTags()
	jsonString := strings.Replace(tags["tx.pools"], `\`, "", -1)
	var tagPools []models.BuySwapPoolTag
	err := json.Unmarshal([]byte(jsonString), &tagPools)
	if err != nil {
		s.logger.Error(err)
		return nil, err
	}

	var mapPoolAddresses = make(map[uint64][]string)
	for _, p := range tagPools {
		var mapAddresses = make(map[string]struct{})
		for _, i := range p.Details.Orders {
			mapAddresses[helpers.RemovePrefix(i.Seller)] = struct{}{}
		}
		for _, i := range p.Sellers {
			mapAddresses[helpers.RemovePrefix(i.Seller)] = struct{}{}
		}

		var list []string
		for a := range mapAddresses {
			list = append(list, a)
		}
		mapPoolAddresses[p.PoolId] = list
	}

	return mapPoolAddresses, err
}

func (s *Service) bigFloatToPipString(f *big.Float) string {
	pip, _ := new(big.Float).Mul(big.NewFloat(1e18), f).Int(nil)
	return pip.String()
}

func (s *Service) updateLiquidityPool(height uint64, lp models.LiquidityPool) error {
	var err error
	var nodeLp *api_pb.SwapPoolResponse

	s.logger.Info(fmt.Sprintf("Updating pool (%d-%d)", lp.FirstCoinId, lp.SecondCoinId))

	if s.chasingMode {
		nodeLp, err = s.nodeApi.SwapPool(lp.FirstCoinId, lp.SecondCoinId, height)
	} else {
		nodeLp, err = s.nodeApi.SwapPool(lp.FirstCoinId, lp.SecondCoinId)
	}

	if err != nil {
		s.logger.Error(err)
		return err
	}

	newLp := &models.LiquidityPool{
		Id:               lp.Id,
		TokenId:          lp.TokenId,
		FirstCoinId:      lp.FirstCoinId,
		SecondCoinId:     lp.SecondCoinId,
		FirstCoinVolume:  nodeLp.Amount0,
		SecondCoinVolume: nodeLp.Amount1,
		Liquidity:        nodeLp.Liquidity,
		UpdatedAtBlockId: height,
	}

	if lp.Id > 0 {
		err = s.Storage.UpdateLiquidityPoolById(newLp)
	} else {
		err = s.Storage.UpdateLiquidityPool(newLp)
	}

	if err != nil {
		s.logger.Error(err)
		return err
	}
	return err
}

func (s *Service) updatePoolsBipLiquidity(lps []models.LiquidityPool) {
	pools, err := s.Storage.GetAll()
	if err != nil {
		s.logger.Error()
		return
	}
	for _, p := range lps {
		liquidityBip := s.swapService.GetPoolLiquidity(pools, p)
		s.logger.Info(fmt.Sprintf("Pool %d Liquidity Bip: %s", p.Id, liquidityBip.Text('f', 18)))
		p.LiquidityBip = s.bigFloatToPipString(liquidityBip)
		err = s.Storage.UpdateLiquidityPool(&p)
		if err != nil {
			s.logger.Error(err)
		}
	}
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
		updateAddressPoolChannel:       make(chan []*api_pb.TransactionResponse, 1),
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
	updateAddressPoolChannel       chan []*api_pb.TransactionResponse
	jobLiquidityPoolTrades         chan []*models.Transaction
	liquidityPoolTradesSaveChannel chan []*models.LiquidityPoolTrade
	chasingMode                    bool
}
