package broadcast

import (
	"context"
	"encoding/json"
	"github.com/MinterTeam/minter-explorer-extender/v2/address"
	"github.com/MinterTeam/minter-explorer-extender/v2/coin"
	"github.com/MinterTeam/minter-explorer-extender/v2/env"
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/centrifugal/gocent"
	"github.com/go-resty/resty/v2"
	"github.com/sirupsen/logrus"
	"os"
	"time"
)

type Service struct {
	client            *gocent.Client
	httpClient        *resty.Client
	ctx               context.Context
	addressRepository *address.Repository
	coinRepository    *coin.Repository
	logger            *logrus.Entry
}

type NodeStatus struct {
	Data struct {
		Version           string    `json:"version"`
		LatestBlockHash   string    `json:"latest_block_hash"`
		LatestAppHash     string    `json:"latest_app_hash"`
		LatestBlockHeight string    `json:"latest_block_height"`
		LatestBlockTime   time.Time `json:"latest_block_time"`
		KeepLastStates    string    `json:"keep_last_states"`
		TotalSlashed      string    `json:"total_slashed"`
		TmStatus          struct {
			NodeInfo struct {
				ProtocolVersion struct {
					P2P   string `json:"p2p"`
					Block string `json:"block"`
					App   string `json:"app"`
				} `json:"protocol_version"`
				ID         string `json:"id"`
				ListenAddr string `json:"listen_addr"`
				Network    string `json:"network"`
				Version    string `json:"version"`
				Channels   string `json:"channels"`
				Moniker    string `json:"moniker"`
				Other      struct {
					TxIndex    string `json:"tx_index"`
					RPCAddress string `json:"rpc_address"`
				} `json:"other"`
			} `json:"node_info"`
			SyncInfo struct {
				LatestBlockHash   string    `json:"latest_block_hash"`
				LatestAppHash     string    `json:"latest_app_hash"`
				LatestBlockHeight string    `json:"latest_block_height"`
				LatestBlockTime   time.Time `json:"latest_block_time"`
				CatchingUp        bool      `json:"catching_up"`
			} `json:"sync_info"`
			ValidatorInfo struct {
				Address string `json:"address"`
				PubKey  struct {
					Type  string `json:"type"`
					Value string `json:"value"`
				} `json:"pub_key"`
				VotingPower string `json:"voting_power"`
			} `json:"validator_info"`
		} `json:"tm_status"`
	} `json:"result"`
}

func NewService(env *env.ExtenderEnvironment, addressRepository *address.Repository, coinRepository *coin.Repository,
	logger *logrus.Entry) *Service {
	wsClient := gocent.New(gocent.Config{
		Addr: env.WsLink,
		Key:  env.WsKey,
	})

	httpClient := resty.New().
		SetHostURL(os.Getenv("NODE_API")).
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
	//channel := `blocks`
	//msg, err := json.Marshal(new(blocks.Resource).Transform(*b))
	//if err != nil {
	//	s.logger.Error(err)
	//}
	//s.publish(channel, []byte(msg))
}

func (s *Service) PublishTransactions(transactions []*models.Transaction) {
	//channel := `transactions`
	//for _, tx := range transactions {
	//	mTransaction := *tx
	//	adr, err := s.addressRepository.FindById(uint(tx.FromAddressID))
	//	mTransaction.FromAddress = &models.Address{Address: adr}
	//	msg, err := json.Marshal(new(transaction.Resource).Transform(mTransaction))
	//	if err != nil {
	//		log.Printf(`Error parse json: %s`, err)
	//	}
	//	s.publish(channel, msg)
	//}
}

func (s *Service) PublishBalances(balances []*models.Balance) {

	//var mapBalances = make(map[uint][]interface{})
	//
	//for _, item := range balances {
	//	symbol, err := s.coinRepository.FindSymbolById(uint(item.CoinID))
	//	if err != nil {
	//		s.logger.Error(err)
	//		continue
	//	}
	//	adr, err := s.addressRepository.FindById(item.AddressID)
	//	if err != nil {
	//		s.logger.Error(err)
	//		continue
	//	}
	//	mBalance := *item
	//	mBalance.Address = &models.Address{Address: adr}
	//	mBalance.Coin = &models.Coin{Symbol: symbol}
	//	res := new(balance.Resource).Transform(mBalance)
	//	mapBalances[item.AddressID] = append(mapBalances[item.AddressID], res)
	//}
	//
	//for addressId, items := range mapBalances {
	//	adr, err := s.addressRepository.FindById(addressId)
	//	if err != nil {
	//		s.logger.Error(err)
	//		continue
	//	}
	//	channel := "Mx" + adr
	//	msg, err := json.Marshal(items)
	//	if err != nil {
	//		log.Printf(`Error parse json: %s`, err)
	//	}
	//	s.publish(channel, []byte(msg))
	//}
}

func (s *Service) PublishStatus() {

	resp, err := s.httpClient.R().
		SetResult(&NodeStatus{}).
		Get("/status")

	if err != nil {
		s.logger.Error(err)
		return
	}

	if resp.IsError() {
		s.logger.Error(err)
		return
	}
	data := resp.Result().(*NodeStatus)
	channel := `status`

	msg, err := json.Marshal(data)
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
