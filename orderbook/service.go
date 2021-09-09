package orderbook

import (
	"fmt"
	"github.com/MinterTeam/minter-explorer-extender/v2/address"
	"github.com/MinterTeam/minter-explorer-extender/v2/liquidity_pool"
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/MinterTeam/minter-explorer-tools/v4/helpers"
	"github.com/MinterTeam/minter-go-sdk/v2/transaction"
	"github.com/MinterTeam/node-grpc-gateway/api_pb"
	"github.com/go-pg/pg/v10"
	"github.com/sirupsen/logrus"
	"strconv"
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

	return &models.Order{
		Id:              orderId,
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

func (s *Service) OrderBookWorker(data <-chan *api_pb.BlockResponse) {
	for b := range data {
		var orderMap sync.Map
		var deleteOrderMap sync.Map
		var list []*models.Order
		var wg sync.WaitGroup
		wg.Add(len(b.Transactions))
		for _, tx := range b.Transactions {
			go func(tx *api_pb.TransactionResponse) {
				switch transaction.Type(tx.Type) {
				case transaction.TypeAddLimitOrder:
					o, err := s.GetOrderDataFromTx(tx)
					if err != nil {
						s.logger.Error(err)
					} else {
						orderMap.Store(o.Id, o)
					}
				case transaction.TypeRemoveLimitOrder:
					txData := new(api_pb.RemoveLimitOrderData)
					if err := tx.GetData().UnmarshalTo(txData); err != nil {
						return
					}
					deleteOrderMap.Store(txData.Id, txData)
				}
				wg.Done()
			}(tx)
		}
		wg.Wait()
		orderMap.Range(func(k, v interface{}) bool {
			list = append(list, v.(*models.Order))
			return true // if false, Range stops
		})
		if len(list) > 0 {
			err := s.Storage.SaveAll(list)
			if err != nil {
				s.logger.Error(err)
			}
		}

		var idForDelete []uint64
		deleteOrderMap.Range(func(k, v interface{}) bool {
			idForDelete = append(idForDelete, k.(uint64))
			return true
		})

		if len(idForDelete) > 0 {
			err := s.Storage.CancelByIdList(idForDelete, models.OrderTypeUserCanceled)
			if err != nil {
				s.logger.Error(err)
			}
		}
	}
}

type Service struct {
	Storage           *Repository
	logger            *logrus.Entry
	addressRepository *address.Repository
	liquidityPool     *liquidity_pool.Service
}

func NewService(db *pg.DB, addressRepository *address.Repository, lpService *liquidity_pool.Service,
	logger *logrus.Entry) *Service {
	return &Service{
		Storage:           NewRepository(db),
		addressRepository: addressRepository,
		liquidityPool:     lpService,
		logger:            logger,
	}
}
