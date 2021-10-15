package balance

import (
	"github.com/MinterTeam/minter-explorer-extender/v2/address"
	"github.com/MinterTeam/minter-explorer-extender/v2/broadcast"
	"github.com/MinterTeam/minter-explorer-extender/v2/coin"
	"github.com/MinterTeam/minter-explorer-extender/v2/env"
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/MinterTeam/minter-explorer-tools/v4/helpers"
	"github.com/MinterTeam/minter-go-sdk/v2/api/grpc_client"
	"github.com/sirupsen/logrus"
	"math"
	"sync"
	"sync/atomic"
)

func (s *Service) BalanceManager() {
	for {
		data := <-s.updateFromResponsesChannel

		//chasingMode, ok := s.chasingMode.Load().(bool)
		//if !ok{
		//	s.logger.Error("chasing mode setup error")
		//	return
		//}
		//if chasingMode {
		//	return
		//}

		_, err, addressesMap := s.addressService.ExtractAddressesFromTransactions(data.Block.Transactions)
		if err != nil {
			s.logger.WithFields(logrus.Fields{
				"block": data.Block.Height,
			}).Error(err)
		}
		listEvent, _ := s.addressService.ExtractAddressesEventsResponse(data.Event)
		for _, i := range listEvent {
			addressesMap[i] = struct{}{}
		}

		var addressesData []string
		for k := range addressesMap {
			addressesData = append(addressesData, k)
		}

		if len(addressesData) > 0 {
			chunksCount := int(math.Ceil(float64(len(addressesData)) / float64(s.env.AddrChunkSize)))
			wg := sync.WaitGroup{}
			wg.Add(chunksCount)
			for i := 0; i < chunksCount; i++ {
				start := s.env.AddrChunkSize * i
				end := start + s.env.AddrChunkSize
				if end > len(addressesData) {
					end = len(addressesData)
				}
				go func(data []string, wg *sync.WaitGroup) {
					err := s.updateAddresses(data)
					if err != nil {
						s.logger.WithFields(logrus.Fields{
							"address_count": len(data),
						}).Error(err)
					}
					wg.Done()
				}(addressesData[start:end], &wg)
			}
			wg.Wait()
		}
	}
}

func (s *Service) UpdateChannel() chan models.BalanceUpdateData {
	return s.updateFromResponsesChannel
}

func (s *Service) SetChasingMode(val bool) {
	s.chasingMode.Store(val)
}

func (s *Service) updateAddresses(list []string) error {
	//chasingMode, ok := s.chasingMode.Load().(bool)
	//if !ok {
	//	chasingMode = false
	//}

	var balances []*models.Balance

	addresses := make([]string, len(list))
	for i, adr := range list {
		addresses[i] = `Mx` + adr
	}

	response, err := s.nodeApi.Addresses(addresses)
	if err != nil {
		return err
	}

	for adr, item := range response.Addresses {
		addressId, err := s.addressService.Storage.FindId(helpers.RemovePrefix(adr))
		if err != nil {
			return err
		}
		for _, val := range item.Balance {
			_, err := s.coinRepository.GetById(uint(val.Coin.Id))
			if err != nil {
				//if !chasingMode {
				//	s.logger.WithFields(logrus.Fields{
				//		"coin":    val.Coin.Id,
				//		"address": adr,
				//	}).Error(err)
				//}
				continue
			}
			balances = append(balances, &models.Balance{
				AddressID: addressId,
				CoinID:    uint(val.Coin.Id),
				Value:     val.Value,
			})
		}
	}

	if len(balances) > 0 {
		var ids []uint
		for _, a := range list {
			addressId, err := s.addressService.Storage.FindId(a)
			if err != nil {
				s.logger.WithFields(logrus.Fields{
					"address": a,
				}).Error(err)
			}
			ids = append(ids, addressId)
		}

		if len(ids) > 0 {
			err = s.repository.DeleteByAddressIds(ids)
			if err != nil {
				if err != nil {
					s.logger.WithFields(logrus.Fields{
						"address_count": len(ids),
					}).Error(err)
				}
			}
		}

		err = s.repository.SaveAll(balances)
		if err != nil {
			return err
		}
		s.broadcastService.BalanceChannel() <- balances
	}

	return err
}

func NewService(env *env.ExtenderEnvironment, repository *Repository, nodeApi *grpc_client.Client,
	addressService *address.Service, coinRepository *coin.Repository, broadcastService *broadcast.Service,
	logger *logrus.Entry) *Service {

	chasingMode := atomic.Value{}
	chasingMode.Store(false)

	return &Service{
		env:                        env,
		repository:                 repository,
		nodeApi:                    nodeApi,
		addressService:             addressService,
		coinRepository:             coinRepository,
		broadcastService:           broadcastService,
		updateFromResponsesChannel: make(chan models.BalanceUpdateData),
		logger:                     logger,
		chasingMode:                chasingMode,
	}
}

type Service struct {
	env                        *env.ExtenderEnvironment
	nodeApi                    *grpc_client.Client
	repository                 *Repository
	addressService             *address.Service
	coinRepository             *coin.Repository
	broadcastService           *broadcast.Service
	wgBalances                 sync.WaitGroup
	logger                     *logrus.Entry
	chasingMode                atomic.Value
	updateFromResponsesChannel chan models.BalanceUpdateData
}
