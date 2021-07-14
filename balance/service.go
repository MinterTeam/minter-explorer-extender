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
)

func (s *Service) BalanceManager() {
	for {
		data := <-s.updateFromResponsesChannel

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

		var ids []uint
		if len(addressesData) > 0 {
			for _, a := range addressesData {
				addressId, err := s.addressService.Storage.FindId(a)
				if err != nil {
					s.logger.WithFields(logrus.Fields{
						"block":   data.Block.Height,
						"address": a,
					}).Error(err)
				}
				ids = append(ids, addressId)
			}
		}
		if len(ids) > 0 {
			chunksCount := int(math.Ceil(float64(len(ids)) / float64(s.env.AddrChunkSize)))
			for i := 0; i < chunksCount; i++ {
				start := s.env.AddrChunkSize * i
				end := start + s.env.AddrChunkSize
				if end > len(ids) {
					end = len(ids)
				}
				err = s.repository.DeleteByAddressIds(ids[start:end])
				if err != nil {
					if err != nil {
						s.logger.WithFields(logrus.Fields{
							"block":         data.Block.Height,
							"address_count": len(ids[start:end]),
						}).Error(err)
					}
				}
			}
		}

		if len(addressesData) > 0 {
			chunksCount := int(math.Ceil(float64(len(addressesData)) / float64(s.env.AddrChunkSize)))
			for i := 0; i < chunksCount; i++ {
				start := s.env.AddrChunkSize * i
				end := start + s.env.AddrChunkSize
				if end > len(addressesData) {
					end = len(addressesData)
				}
				s.channelUpdate <- addressesData[start:end]
			}
		}
	}
}

func (s *Service) BalanceUpdater() {
	var err error
	for {
		data := <-s.channelUpdate
		if len(data) > 0 {
			err = s.updateAddresses(data)
			if err != nil {
				s.logger.Error(err)
			}
		}
	}
}

func (s *Service) UpdateChannel() chan models.BalanceUpdateData {
	return s.updateFromResponsesChannel
}

func (s *Service) SetChasingMode(chasingMode bool) {
	s.chasingMode = chasingMode
}

func (s *Service) updateAddresses(list []string) error {
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
	return &Service{
		env:                        env,
		repository:                 repository,
		nodeApi:                    nodeApi,
		addressService:             addressService,
		coinRepository:             coinRepository,
		broadcastService:           broadcastService,
		updateFromResponsesChannel: make(chan models.BalanceUpdateData),
		channelUpdate:              make(chan []string),
		logger:                     logger,
		chasingMode:                false,
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
	chasingMode                bool
	updateFromResponsesChannel chan models.BalanceUpdateData
	channelUpdate              chan []string
}
