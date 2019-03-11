package broadcast

import (
	"context"
	"encoding/json"
	"github.com/MinterTeam/minter-explorer-extender/address"
	"github.com/MinterTeam/minter-explorer-extender/coin"
	"github.com/MinterTeam/minter-explorer-extender/models"
	"github.com/centrifugal/gocent"
	"log"
)

type Service struct {
	client            *gocent.Client
	ctx               context.Context
	addressRepository *address.Repository
	coinRepository    *coin.Repository
}

type Balance struct {
	Address string
	Coin    string
	Value   string
}

type Tx struct {
	From string
	Hash string
	Data json.RawMessage
}

func NewService(env *models.ExtenderEnvironment, addressRepository *address.Repository, coinRepository *coin.Repository) *Service {
	wsClient := gocent.New(gocent.Config{
		Addr: env.WsLink,
		Key:  env.WsKey,
	})

	return &Service{
		client:            wsClient,
		ctx:               context.Background(),
		addressRepository: addressRepository,
		coinRepository:    coinRepository,
	}
}

func (s *Service) PublishBlock(b *models.Block) {
	ch := `blocks`
	msg, err := json.Marshal(b)
	if err != nil {
		log.Printf(`Error parse json: %s`, err)
	}
	s.publish(ch, []byte(msg))
}

func (s *Service) PublishTransactions(txs []*models.Transaction) {
	ch := `transactions`
	for _, tx := range txs {
		adr, err := s.addressRepository.FindById(tx.FromAddressID)
		adr = "Mx" + adr
		msg, err := json.Marshal(Tx{
			From: adr,
			Hash: "Mt" + tx.Hash,
			Data: tx.Data,
		})
		if err != nil {
			log.Printf(`Error parse json: %s`, err)
		}
		s.publish(ch, []byte(msg))
	}
}

func (s *Service) PublishBalances(balances []*models.Balance) {
	for _, balance := range balances {
		adr, err := s.addressRepository.FindById(balance.AddressID)
		ch := "Mx" + adr
		symbol, err := s.coinRepository.FindSymbolById(balance.CoinID)
		if err != nil {
			log.Printf(err.Error())
		}
		msg, err := json.Marshal(Balance{
			Address: ch,
			Coin:    symbol,
			Value:   balance.Value,
		})
		if err != nil {
			log.Printf(`Error parse json: %s`, err)
		}
		s.publish(ch, []byte(msg))
	}
}

func (s *Service) publish(ch string, msg []byte) {
	err := s.client.Publish(s.ctx, ch, msg)
	if err != nil {
		log.Printf(`Error calling publish: %s`, err)
	}
}
