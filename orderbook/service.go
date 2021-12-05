package orderbook

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/MinterTeam/minter-explorer-extender/v2/address"
	"github.com/MinterTeam/minter-explorer-extender/v2/liquidity_pool"
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/MinterTeam/minter-explorer-tools/v4/helpers"
	"github.com/MinterTeam/minter-go-sdk/v2/transaction"
	"github.com/MinterTeam/node-grpc-gateway/api_pb"
	"github.com/go-pg/pg/v10"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
	"math/big"
	"strconv"
	"strings"
	"sync"
)

func (s *Service) GetOrderDataFromTx(tx *api_pb.TransactionResponse) (*models.Order, error) {
	txData := new(api_pb.AddLimitOrderData)
	if err := tx.GetData().UnmarshalTo(txData); err != nil {
		return nil, err
	}
	txTags := tx.GetTags()

	addressId, err := s.addressRepository.FindIdOrCreate(helpers.RemovePrefix(tx.From))
	if err != nil {
		return nil, err
	}

	orderId, err := strconv.ParseUint(txTags["tx.order_id"], 10, 64)
	if err != nil {
		return nil, err
	}

	var lpId uint64
	if txTags["tx.pool_id"] != "" {
		lpId, err = strconv.ParseUint(txTags["tx.pool_id"], 10, 64)
		if err != nil {
			return nil, err
		}
	} else {
		var fc, sc uint64
		if txData.CoinToSell.Id < txData.CoinToBuy.Id {
			fc = txData.CoinToSell.Id
			sc = txData.CoinToBuy.Id
		} else {
			fc = txData.CoinToBuy.Id
			sc = txData.CoinToSell.Id
		}
		lp, err := s.liquidityPool.GetPoolByPairString(fmt.Sprintf("%d-%d", fc, sc))
		if err != nil {
			return nil, err
		}
		lpId = lp.Id
	}

	sell, ok := big.NewFloat(0).SetString(txData.ValueToSell)
	if !ok {
		return nil, errors.New("can't convert to big.int")
	}
	buy, ok := big.NewFloat(0).SetString(txData.ValueToBuy)
	if !ok {
		return nil, errors.New("can't convert to big.int ")
	}
	price := sell.Quo(sell, buy)

	if price.Cmp(big.NewFloat(0)) <= 0 {
		s.logger.Error("zero price")
	}

	return &models.Order{
		Id:              orderId,
		Price:           price.Text('f', 18),
		Status:          models.OrderTypeNew,
		AddressId:       uint64(addressId),
		LiquidityPoolId: lpId,
		CoinSellId:      txData.CoinToSell.Id,
		CoinSellVolume:  txData.ValueToSell,
		CoinBuyId:       txData.CoinToBuy.Id,
		CoinBuyVolume:   txData.ValueToBuy,
		CreatedAtBlock:  tx.Height,
	}, nil
}

func (s *Service) UpdateOrderBookWorker(data <-chan []models.TxTagDetailsOrder) {
	for orders := range data {
		s.updateOrders(orders)
	}
}

func (s *Service) updateOrders(orders []models.TxTagDetailsOrder) {
	mapId := make(map[uint64]struct{})
	for _, o := range orders {
		mapId[o.Id] = struct{}{}
	}

	var listId []uint64
	for id := range mapId {
		listId = append(listId, id)
	}

	orderList, err := s.Storage.GetAllById(listId)
	if err != nil {
		s.logger.Error(err)
		return
	}
	mapOrders := make(map[uint64]models.Order)
	for _, o := range orderList {
		mapOrders[o.Id] = o
	}

	for _, o := range orders {
		buyValue, ok := big.NewInt(0).SetString(o.Buy, 10)
		if !ok {
			s.logger.WithFields(logrus.Fields{
				"order_id": o.Id,
				"buy":      o.Buy,
				"sell":     o.Sell,
				"address":  o.Seller,
			}).Error("can't parse big.Int")
			continue
		}
		sellValue, ok := big.NewInt(0).SetString(o.Sell, 10)
		if !ok {
			s.logger.WithFields(logrus.Fields{
				"order_id": o.Id,
				"buy":      o.Buy,
				"sell":     o.Sell,
				"address":  o.Seller,
			}).Error("can't parse big.Int")
			continue
		}

		orderBuyValue, ok := big.NewInt(0).SetString(mapOrders[o.Id].CoinBuyVolume, 10)
		if !ok {
			s.logger.WithFields(logrus.Fields{
				"order_id": o.Id,
				"buy":      mapOrders[o.Id].CoinBuyVolume,
				"sell":     mapOrders[o.Id].CoinSellVolume,
			}).Error("can't parse big.Int")
			continue
		}
		orderSellValue, ok := big.NewInt(0).SetString(mapOrders[o.Id].CoinSellVolume, 10)
		if !ok {
			s.logger.WithFields(logrus.Fields{
				"order_id": o.Id,
				"buy":      mapOrders[o.Id].CoinBuyVolume,
				"sell":     mapOrders[o.Id].CoinSellVolume,
			}).Error("can't parse big.Int")
			continue
		}

		newBuyValue := big.NewInt(0)
		newBuyValue = newBuyValue.Sub(orderBuyValue, buyValue)

		newSellValue := big.NewInt(0)
		newSellValue = newSellValue.Sub(orderSellValue, sellValue)

		status := models.OrderTypeActive
		if newBuyValue.Cmp(big.NewInt(0)) <= 0 || newSellValue.Cmp(big.NewInt(0)) <= 0 {
			status = models.OrderTypeFilled
		}

		if newBuyValue.Cmp(big.NewInt(0)) < 0 || newSellValue.Cmp(big.NewInt(0)) < 0 {
			s.logger.WithFields(logrus.Fields{
				"order_id": o.Id,
				"buy":      newBuyValue.String(),
				"sell":     newSellValue.String(),
			}).Error("negative value")
		}

		if mapOrders[o.Id].Price == "0" {
			s.logger.WithFields(logrus.Fields{
				"order_id": o.Id,
				"buy":      newBuyValue.String(),
				"sell":     newSellValue.String(),
			}).Error("zero price")
		}

		mapOrders[o.Id] = models.Order{
			Id:              o.Id,
			Price:           mapOrders[o.Id].Price,
			AddressId:       mapOrders[o.Id].AddressId,
			LiquidityPoolId: mapOrders[o.Id].LiquidityPoolId,
			CoinSellId:      mapOrders[o.Id].CoinSellId,
			CoinSellVolume:  newSellValue.String(),
			CoinBuyId:       mapOrders[o.Id].CoinBuyId,
			CoinBuyVolume:   newBuyValue.String(),
			CreatedAtBlock:  mapOrders[o.Id].CreatedAtBlock,
			Status:          status,
		}
	}

	var list []models.Order
	for _, o := range mapOrders {
		list = append(list, o)
	}

	err = s.Storage.UpdateOrders(&list)
	if err != nil {
		s.logger.Error(err)
	}
}

func (s *Service) OrderBookWorker(data <-chan *api_pb.BlockResponse) {
	for b := range data {
		if len(b.Transactions) < 1 {
			continue
		}
		var orderMap sync.Map
		var deleteOrderMap sync.Map
		var updateOrderMap sync.Map
		var wg sync.WaitGroup

		for _, tx := range b.Transactions {
			if tx.Log != "" {
				continue
			}
			wg.Add(1)
			go func(tx *api_pb.TransactionResponse) {
				switch transaction.Type(tx.Type) {
				case transaction.TypeAddLimitOrder:
					o, err := s.GetOrderDataFromTx(tx)
					if err != nil {
						s.logger.Error(err)
					} else {
						orderMap.Store(o.Id, o)
					}
					tags := tx.GetTags()
					if tags["tx.commission_conversion"] == "pool" {
						jsonString := strings.Replace(tags["tx.commission_details"], `\`, "", -1)
						var tag models.TagCommissionDetails
						err = json.Unmarshal([]byte(jsonString), &tag)
						if err != nil {
							s.logger.Error(err)
						} else {
							for _, orderDetail := range tag.Details.Orders {
								updateOrderMap.Store(fmt.Sprintf("%s", uuid.New()), orderDetail)
							}
						}
					}
				case transaction.TypeRemoveLimitOrder:
					txData := new(api_pb.RemoveLimitOrderData)
					if err := tx.GetData().UnmarshalTo(txData); err != nil {
						s.logger.Error(err)
					} else {
						deleteOrderMap.Store(txData.Id, txData)
					}
					tags := tx.GetTags()
					if tags["tx.commission_conversion"] == "pool" {
						jsonString := strings.Replace(tags["tx.commission_details"], `\`, "", -1)
						var tag models.TagCommissionDetails
						err := json.Unmarshal([]byte(jsonString), &tag)
						if err != nil {
							s.logger.Error(err)
						} else {
							for _, orderDetail := range tag.Details.Orders {
								updateOrderMap.Store(fmt.Sprintf("%s", uuid.New()), orderDetail)
							}
						}
					}
				case transaction.TypeBuySwapPool,
					transaction.TypeSellSwapPool,
					transaction.TypeSellAllSwapPool:
					tags := tx.GetTags()
					jsonString := strings.Replace(tags["tx.pools"], `\`, "", -1)
					var tagPools []models.BuySwapPoolTag
					err := json.Unmarshal([]byte(jsonString), &tagPools)
					if err != nil {
						s.logger.WithFields(logrus.Fields{
							"json":  jsonString,
							"tx":    tx.Hash,
							"block": tx.Height,
						}).Error(err)
					} else {
						for _, p := range tagPools {
							for _, tagOrderDetail := range p.Details.Orders {
								updateOrderMap.Store(fmt.Sprintf("%s", uuid.New()), tagOrderDetail)
							}
						}
					}
					if tags["tx.commission_conversion"] == "pool" {
						jsonString = strings.Replace(tags["tx.commission_details"], `\`, "", -1)
						var tag models.TagCommissionDetails
						err = json.Unmarshal([]byte(jsonString), &tag)
						if err != nil {
							s.logger.WithFields(logrus.Fields{
								"json":  jsonString,
								"tx":    tx.Hash,
								"block": tx.Height,
							}).Error(err)
						} else {
							for _, orderDetail := range tag.Details.Orders {
								updateOrderMap.Store(fmt.Sprintf("%s", uuid.New()), orderDetail)
							}
						}
					}
				default:
					tags := tx.GetTags()
					if tags["tx.commission_conversion"] == "pool" {
						jsonString := strings.Replace(tags["tx.commission_details"], `\`, "", -1)
						var tag models.TagCommissionDetails
						err := json.Unmarshal([]byte(jsonString), &tag)
						if err != nil {
							s.logger.WithFields(logrus.Fields{
								"json":  jsonString,
								"tx":    tx.Hash,
								"block": tx.Height,
							}).Error(err)
						} else {
							for _, orderDetail := range tag.Details.Orders {
								updateOrderMap.Store(fmt.Sprintf("%s", uuid.New()), orderDetail)
							}
						}
					}
				}
				wg.Done()
			}(tx)
		}
		wg.Wait()

		//var list []*models.Order
		newOrdersMap := make(map[int][]*models.Order)
		index := 0
		orderMap.Range(func(k, v interface{}) bool {
			if newOrdersMap[index] == nil {
				newOrdersMap[index] = []*models.Order{}
			}
			newOrdersMap[index] = append(newOrdersMap[index], v.(*models.Order))
			if len(newOrdersMap[index]) > 50000 {
				index++
			}
			return true
		})
		if len(newOrdersMap) > 0 {
			for _, orders := range newOrdersMap {
				err := s.Storage.SaveAll(orders)
				if err != nil {
					s.logger.Error(err)
				}
			}
		}

		var idForDelete []uint64
		deleteOrderMap.Range(func(k, v interface{}) bool {
			idForDelete = append(idForDelete, k.(uint64))
			return true
		})

		uom := make(map[int][]models.TxTagDetailsOrder)
		index = 0
		updateOrderMap.Range(func(k, v interface{}) bool {
			if uom[index] == nil {
				uom[index] = []models.TxTagDetailsOrder{}
			}
			uom[index] = append(uom[index], v.(models.TxTagDetailsOrder))
			if len(uom[index]) > 50000 {
				index++
			}
			return true
		})

		if len(uom) > 0 {
			for _, orders := range uom {
				s.updateOrders(orders)
			}
		}

		if len(idForDelete) > 0 {
			err := s.Storage.CancelByIdList(idForDelete, models.OrderTypeCanceled)
			if err != nil {
				s.logger.Error(err)
			}
		}
	}
}

type Service struct {
	Storage            *Repository
	logger             *logrus.Entry
	addressRepository  *address.Repository
	liquidityPool      *liquidity_pool.Service
	updateOrderChannel chan []models.TxTagDetailsOrder
}

func (s *Service) UpdateOrderChannel() chan []models.TxTagDetailsOrder {
	return s.updateOrderChannel
}

func NewService(db *pg.DB, addressRepository *address.Repository, lpService *liquidity_pool.Service,
	logger *logrus.Entry) *Service {
	return &Service{
		updateOrderChannel: make(chan []models.TxTagDetailsOrder),
		Storage:            NewRepository(db),
		addressRepository:  addressRepository,
		liquidityPool:      lpService,
		logger:             logger,
	}
}
