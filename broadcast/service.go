package broadcast

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/MinterTeam/minter-explorer-api/v2/balance"
	"github.com/MinterTeam/minter-explorer-api/v2/blocks"
	"github.com/MinterTeam/minter-explorer-extender/v2/address"
	"github.com/MinterTeam/minter-explorer-extender/v2/coin"
	"github.com/MinterTeam/minter-explorer-extender/v2/env"
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/MinterTeam/minter-go-sdk/v2/api"
	"github.com/MinterTeam/minter-go-sdk/v2/api/grpc_client"
	"github.com/centrifugal/gocent"
	"github.com/sirupsen/logrus"
	"log"
	"time"
)

type Service struct {
	client            *gocent.Client
	nodeClient        *grpc_client.Client
	ctx               context.Context
	addressRepository *address.Repository
	coinRepository    *coin.Repository
	logger            *logrus.Entry
	chasingMode       bool
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
		chasingMode:       false,
	}
}

func (s *Service) SetChasingMode(chasingMode bool) {
	s.chasingMode = chasingMode
}

func (s *Service) PublishBlock(b models.Block) {
	channel := `blocks`
	msg, err := json.Marshal(new(blocks.Resource).Transform(b))
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

		if err != nil {
			s.logger.Error(err)
			continue
		}

		mTransaction.FromAddress = &models.Address{Address: adr}
		msg, err := json.Marshal(new(TransactionResource).Transform(mTransaction))
		if err != nil {
			s.logger.Error(err)
			continue
		}
		s.publish(channel, msg)
	}
}

func (s *Service) PublishBalances(balances []*models.Balance) {
	defer func() {
		if err := recover(); err != nil {
			var list []models.Balance
			for _, b := range balances {
				list = append(list, *b)
			}
			s.logger.WithField("balances", list).Error(err)
		}
	}()

	var mapBalances = make(map[uint][]interface{})

	for _, item := range balances {
		c, err := s.coinRepository.GetById(item.CoinID)
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
		mBalance.Coin = c
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

	if s.chasingMode {
		return
	}
	start := time.Now()
	status, err := s.nodeClient.Status()
	if err != nil {
		s.logger.Error(err)
		return
	}
	elapsed := time.Since(start)
	s.logger.Info(fmt.Sprintf("Status data getting time: %s", elapsed))

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

func (s *Service) PublishCommissions(data api.Event) {
	channel := `commissions`
	msg, err := json.Marshal(data)
	if err != nil {
		s.logger.Error(err)
	}
	s.publish(channel, msg)
}
