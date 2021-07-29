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
	"github.com/MinterTeam/minter-go-sdk/v2/transaction"
	"github.com/MinterTeam/node-grpc-gateway/api_pb"
	"github.com/centrifugal/gocent"
	"github.com/sirupsen/logrus"
	"log"
	"math/big"
)

type Service struct {
	client              *gocent.Client
	nodeClient          *grpc_client.Client
	ctx                 context.Context
	addressRepository   *address.Repository
	coinRepository      *coin.Repository
	logger              *logrus.Entry
	stakeChannel        chan *api_pb.TransactionResponse
	chasingMode         bool
	blockChannel        chan models.Block
	transactionsChannel chan []*models.Transaction
	balanceChannel      chan []*models.Balance
	commissionsChannel  chan api.Event
}

func NewService(env *env.ExtenderEnvironment, addressRepository *address.Repository, coinRepository *coin.Repository,
	nodeClient *grpc_client.Client, logger *logrus.Entry) *Service {
	wsClient := gocent.New(gocent.Config{
		Addr: env.WsLink,
		Key:  env.WsKey,
	})

	return &Service{
		client:              wsClient,
		nodeClient:          nodeClient,
		ctx:                 context.Background(),
		addressRepository:   addressRepository,
		coinRepository:      coinRepository,
		commissionsChannel:  make(chan api.Event),
		stakeChannel:        make(chan *api_pb.TransactionResponse),
		balanceChannel:      make(chan []*models.Balance),
		transactionsChannel: make(chan []*models.Transaction),
		blockChannel:        make(chan models.Block),
		logger:              logger,
		chasingMode:         false,
	}
}

func (s *Service) Manager() {
	for {
		select {
		case b := <-s.blockChannel:
			s.PublishBlock(b)
			s.PublishStatus()
		case txs := <-s.transactionsChannel:
			go s.PublishTransactions(txs)
		case b := <-s.balanceChannel:
			if b != nil {

			}
			//TODO: enable in prod
			//go s.PublishBalances(b)
		case tx := <-s.stakeChannel:
			go s.PublishStake(tx)
		case c := <-s.commissionsChannel:
			s.PublishCommissions(c)
		}
	}
}

func (s *Service) CommissionsChannel() chan api.Event {
	return s.commissionsChannel
}

func (s *Service) StakeChannel() chan *api_pb.TransactionResponse {
	return s.stakeChannel
}

func (s *Service) BalanceChannel() chan []*models.Balance {
	return s.balanceChannel
}

func (s *Service) TransactionsChannel() chan []*models.Transaction {
	return s.transactionsChannel
}

func (s *Service) BlockChannel() chan models.Block {
	return s.blockChannel
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
	channelCut := `transactions_100`
	count := 0
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

		if count < 100 {
			s.publish(channelCut, msg)
		}
		count++
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
func (s *Service) PublishCommissions(data api.Event) {
	channel := `commissions`
	msg, err := json.Marshal(data)
	if err != nil {
		s.logger.Error(err)
	}
	s.publish(channel, msg)
}

func (s *Service) PublishStake(tx *api_pb.TransactionResponse) {
	var val *big.Int
	channel := `stake/%s`
	addressCache := make(map[string]*big.Int)

	if transaction.Type(tx.Type) == transaction.TypeDelegate {
		txData := new(api_pb.DelegateData)
		if err := tx.Data.UnmarshalTo(txData); err != nil {
			s.logger.Error(err)
			return
		}
		val, _ = big.NewInt(0).SetString(txData.Value, 10)
	}

	if transaction.Type(tx.Type) == transaction.TypeUnbond {
		txData := new(api_pb.UnbondData)
		if err := tx.Data.UnmarshalTo(txData); err != nil {
			s.logger.Error(err)
			return
		}
		val, _ = big.NewInt(0).SetString(txData.Value, 10)
	}

	if tx.Height%120 == 0 {
		if len(addressCache) > 0 {
			for a := range addressCache {
				s.publish(fmt.Sprintf(channel, tx.From), []byte(fmt.Sprintf(`{"data" : "%s"}`, a)))
				delete(addressCache, a)
			}
		}
		s.publish(fmt.Sprintf(channel, tx.From), []byte(fmt.Sprintf(`{"data" : "%s"}`, val.String())))
	} else {

		if addressCache[tx.From] == nil {
			addressCache[tx.From] = big.NewInt(0)
		}

		if transaction.Type(tx.Type) == transaction.TypeDelegate {
			addressCache[tx.From].Add(addressCache[tx.From], val)
		}

		if transaction.Type(tx.Type) == transaction.TypeUnbond {
			addressCache[tx.From].Sub(addressCache[tx.From], val)
		}
	}
}
func (s *Service) publish(ch string, msg []byte) {
	err := s.client.Publish(s.ctx, ch, msg)
	if err != nil {
		s.logger.Warn(err)
	}
}
