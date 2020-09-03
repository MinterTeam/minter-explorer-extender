package broadcast

import (
	"context"
	"encoding/json"
	"github.com/MinterTeam/minter-explorer-api/balance"
	"github.com/MinterTeam/minter-explorer-api/blocks"
	"github.com/MinterTeam/minter-explorer-extender/v2/address"
	"github.com/MinterTeam/minter-explorer-extender/v2/coin"
	"github.com/MinterTeam/minter-explorer-extender/v2/env"
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/MinterTeam/minter-go-sdk/v2/api/grpc_client"
	"github.com/centrifugal/gocent"
	"github.com/sirupsen/logrus"
	"log"
)

type Service struct {
	client            *gocent.Client
	nodeClient        *grpc_client.Client
	ctx               context.Context
	addressRepository *address.Repository
	coinRepository    *coin.Repository
	logger            *logrus.Entry
}

func NewService(env *env.ExtenderEnvironment, addressRepository *address.Repository, coinRepository *coin.Repository,
	nodeClient *grpc_client.Client, logger *logrus.Entry) *Service {
	wsClient := gocent.New(gocent.Config{
		Addr: env.WsLink,
		Key:  env.WsKey,
	})

	return &Service{
		client:            wsClient,
		nodeClient:        nodeClient,
		ctx:               context.Background(),
		addressRepository: addressRepository,
		coinRepository:    coinRepository,
		logger:            logger,
	}
}

func (s *Service) PublishBlock(b *models.Block) {
	channel := `blocks`
	msg, err := json.Marshal(new(blocks.Resource).Transform(*b))
	if err != nil {
		s.logger.Error(err)
	}
	s.publish(channel, msg)
}

func (s *Service) PublishTransactions(transactions []*models.Transaction) {
	channel := `transactions`
	for _, tx := range transactions {
		mTransaction := *tx
		adr, err := s.addressRepository.FindById(uint(tx.FromAddressID))
		mTransaction.FromAddress = &models.Address{Address: adr}
		msg, err := json.Marshal(new(TransactionResource).Transform(mTransaction))
		if err != nil {
			log.Printf(`Error parse json: %s`, err)
		}
		s.publish(channel, msg)
	}
}

func (s *Service) PublishBalances(balances []*models.Balance) {

	var mapBalances = make(map[uint][]interface{})

	for _, item := range balances {
		symbol, err := s.coinRepository.FindSymbolById(uint(item.CoinID))
		if err != nil {
			s.logger.Error(err)
			continue
		}
		adr, err := s.addressRepository.FindById(item.AddressID)
		if err != nil {
			s.logger.Error(err)
			continue
		}
		mBalance := *item
		mBalance.Address = &models.Address{Address: adr}
		mBalance.Coin = &models.Coin{Symbol: symbol}
		res := new(balance.Resource).Transform(mBalance)
		mapBalances[item.AddressID] = append(mapBalances[item.AddressID], res)
	}

	for addressId, items := range mapBalances {
		adr, err := s.addressRepository.FindById(addressId)
		if err != nil {
			s.logger.Error(err)
			continue
		}
		channel := "Mx" + adr
		msg, err := json.Marshal(items)
		if err != nil {
			log.Printf(`Error parse json: %s`, err)
		}
		s.publish(channel, msg)
	}
}

func (s *Service) PublishStatus() {

	status, err := s.nodeClient.Status()

	if err != nil {
		s.logger.Error(err)
		return
	}

	channel := `status`

	msg, err := json.Marshal(status)
	if err != nil {
		s.logger.Error(err)
	}
	s.publish(channel, msg)
}

func (s *Service) publish(ch string, msg []byte) {
	err := s.client.Publish(s.ctx, ch, msg)
	if err != nil {
		s.logger.Warn(err)
	}
}
