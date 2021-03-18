package balance

import (
	"github.com/MinterTeam/minter-explorer-extender/v2/address"
	"github.com/MinterTeam/minter-explorer-extender/v2/broadcast"
	"github.com/MinterTeam/minter-explorer-extender/v2/coin"
	"github.com/MinterTeam/minter-explorer-extender/v2/env"
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/MinterTeam/minter-explorer-tools/v4/helpers"
	"github.com/MinterTeam/minter-go-sdk/v2/api/grpc_client"
	"github.com/MinterTeam/node-grpc-gateway/api_pb"
	"github.com/sirupsen/logrus"
	"math"
	"sync"
)

func (s *Service) BalanceManager() {
	var err error
	for {

		data := <-s.channelDataForUpdate

		block, ok := data.(*api_pb.BlockResponse)
		if ok {
			err = s.updateBalancesByBlockData(block)
			if err != nil {
				s.logger.Error(err)
			}
			continue
		}

		event, ok := data.(*api_pb.EventsResponse)
		if ok {
			err = s.updateBalancesByEventData(event)
			if err != nil {
				s.logger.Error(err)
			}
			continue
		}

		s.logger.Error("wrong data for balance update")
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

func (s *Service) ChannelDataForUpdate() chan interface{} {
	return s.channelDataForUpdate
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

	var ids []uint

	for adr, item := range response.Addresses {
		addressId, err := s.addressService.Storage.FindId(helpers.RemovePrefix(adr))
		if err != nil {
			return err
		}
		ids = append(ids, addressId)
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

	err = s.repository.DeleteByAddressIds(ids)
	if err != nil {
		return err
	}

	err = s.repository.SaveAll(balances)
	if err != nil {
		return err
	}
	s.broadcastService.PublishBalances(balances)
	return err
}

func (s *Service) updateBalancesByBlockData(block *api_pb.BlockResponse) error {
	list, err, _ := s.addressService.ExtractAddressesFromTransactions(block.Transactions)
	if err != nil {
		return err
	}
	if len(list) > 0 {
		chunksCount := int(math.Ceil(float64(len(list)) / float64(s.env.AddrChunkSize)))
		for i := 0; i < chunksCount; i++ {
			start := s.env.AddrChunkSize * i
			end := start + s.env.AddrChunkSize
			if end > len(list) {
				end = len(list)
			}
			s.channelUpdate <- list[start:end]
		}
	}
	return nil
}

func (s *Service) updateBalancesByEventData(event *api_pb.EventsResponse) error {
	list, _ := s.addressService.ExtractAddressesEventsResponse(event)
	if len(list) > 0 {
		return s.updateAddresses(list)
	}
	return nil
}

func NewService(env *env.ExtenderEnvironment, repository *Repository, nodeApi *grpc_client.Client, addressService *address.Service, coinRepository *coin.Repository, broadcastService *broadcast.Service, logger *logrus.Entry) *Service {
	return &Service{
		env:                  env,
		repository:           repository,
		nodeApi:              nodeApi,
		addressService:       addressService,
		coinRepository:       coinRepository,
		broadcastService:     broadcastService,
		channelDataForUpdate: make(chan interface{}),
		channelUpdate:        make(chan []string),
		logger:               logger,
		chasingMode:          false,
	}
}

type Service struct {
	env                  *env.ExtenderEnvironment
	nodeApi              *grpc_client.Client
	repository           *Repository
	addressService       *address.Service
	coinRepository       *coin.Repository
	broadcastService     *broadcast.Service
	wgBalances           sync.WaitGroup
	logger               *logrus.Entry
	chasingMode          bool
	channelDataForUpdate chan interface{}
	channelUpdate        chan []string
}
