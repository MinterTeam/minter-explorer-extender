package broadcast

import (
	"context"
	"encoding/json"
	"github.com/MinterTeam/minter-explorer-api/balance"
	"github.com/MinterTeam/minter-explorer-api/blocks"
	"github.com/MinterTeam/minter-explorer-api/transaction"
	"github.com/MinterTeam/minter-explorer-extender/v2/address"
	"github.com/MinterTeam/minter-explorer-extender/v2/coin"
	"github.com/MinterTeam/minter-explorer-extender/v2/env"
	"github.com/MinterTeam/minter-explorer-tools/v4/models"
	"github.com/centrifugal/gocent"
	"github.com/go-resty/resty/v2"
	"github.com/sirupsen/logrus"
	"log"
)

type Service struct {
	client            *gocent.Client
	httpClient        *resty.Client
	ctx               context.Context
	addressRepository *address.Repository
	coinRepository    *coin.Repository
	logger            *logrus.Entry
}

type TotalSlashesResponse struct {
	Result json.Number `json:"result"`
}
type TSResponse struct {
	Result string `json:"result"`
}

func NewService(env *env.ExtenderEnvironment, addressRepository *address.Repository, coinRepository *coin.Repository,
	logger *logrus.Entry) *Service {
	wsClient := gocent.New(gocent.Config{
		Addr: env.WsLink,
		Key:  env.WsKey,
	})

	httpClient := resty.New().
		SetHostURL("http://195.201.244.41:8841").
		SetHeader("Accept", "application/json")

	return &Service{
		client:            wsClient,
		httpClient:        httpClient,
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
	s.publish(channel, []byte(msg))
}

func (s *Service) PublishTransactions(transactions []*models.Transaction) {
	channel := `transactions`
	for _, tx := range transactions {
		mTransaction := *tx
		adr, err := s.addressRepository.FindById(tx.FromAddressID)
		mTransaction.FromAddress = &models.Address{Address: adr}
		msg, err := json.Marshal(new(transaction.Resource).Transform(mTransaction))
		if err != nil {
			log.Printf(`Error parse json: %s`, err)
		}
		s.publish(channel, []byte(msg))
	}
}

func (s *Service) PublishBalances(balances []*models.Balance) {

	var mapBalances = make(map[uint64][]interface{})

	for _, item := range balances {
		symbol, err := s.coinRepository.FindSymbolById(item.CoinID)
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
		s.publish(channel, []byte(msg))
	}
}

func (s *Service) PublishTotalSlashes() {

	resp, err := s.httpClient.R().
		SetResult(&TotalSlashesResponse{}).
		Get("/total_slashed")

	if err != nil {
		s.logger.Error(err)
		return
	}

	if resp.IsError() {
		s.logger.Error(err)
		return
	}
	data := resp.Result().(*TotalSlashesResponse)
	channel := `total_slashed`
	msg, err := json.Marshal(TSResponse{
		Result: data.Result.String(),
	})
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
