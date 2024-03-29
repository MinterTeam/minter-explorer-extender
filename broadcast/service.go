package broadcast

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/MinterTeam/minter-explorer-api/v2/blocks"
	"github.com/MinterTeam/minter-explorer-extender/v2/address"
	"github.com/MinterTeam/minter-explorer-extender/v2/centrifugopb"
	"github.com/MinterTeam/minter-explorer-extender/v2/coin"
	"github.com/MinterTeam/minter-explorer-extender/v2/env"
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/MinterTeam/minter-go-sdk/v2/api/grpc_client"
	"github.com/MinterTeam/minter-go-sdk/v2/transaction"
	"github.com/MinterTeam/node-grpc-gateway/api_pb"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"log"
	"math/big"
	"os"
	"sync/atomic"
)

type keyAuth struct {
	key string
}

func (t keyAuth) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": "apikey " + t.key,
	}, nil
}

func (t keyAuth) RequireTransportSecurity() bool {
	return false
}

type Service struct {
	client              centrifugopb.CentrifugoApiClient
	nodeClient          *grpc_client.Client
	addressRepository   *address.Repository
	coinRepository      *coin.Repository
	logger              *logrus.Entry
	stakeChannel        chan *api_pb.TransactionResponse
	chasingMode         atomic.Value
	blockChannel        chan models.Block
	transactionsChannel chan []*models.Transaction
	balanceChannel      chan []*models.Balance
	commissionsChannel  chan *api_pb.UpdateCommissionsEvent
}

func NewService(env *env.ExtenderEnvironment, addressRepository *address.Repository, coinRepository *coin.Repository,
	nodeClient *grpc_client.Client, logger *logrus.Entry) *Service {

	//wsClient := gocent.New(gocent.Config{
	//	Addr: env.WsLink,
	//	Key:  env.WsKey,
	//})

	chasingMode := atomic.Value{}
	chasingMode.Store(false)

	conn, err := grpc.Dial("centrifugo:10000", grpc.WithInsecure(), grpc.WithPerRPCCredentials(keyAuth{os.Getenv("CENTRIFUGO_SECRET")}))

	if err != nil {
		log.Fatalln(err)
	}
	wsClient := centrifugopb.NewCentrifugoApiClient(conn)

	return &Service{
		client:              wsClient,
		nodeClient:          nodeClient,
		addressRepository:   addressRepository,
		coinRepository:      coinRepository,
		commissionsChannel:  make(chan *api_pb.UpdateCommissionsEvent),
		stakeChannel:        make(chan *api_pb.TransactionResponse),
		balanceChannel:      make(chan []*models.Balance),
		transactionsChannel: make(chan []*models.Transaction),
		blockChannel:        make(chan models.Block),
		logger:              logger,
		chasingMode:         chasingMode,
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
			go s.PublishBalances(b)
		case tx := <-s.stakeChannel:
			go s.PublishStake(tx)
		case c := <-s.commissionsChannel:
			s.PublishCommissions(c)
		}
	}
}

func (s *Service) CommissionsChannel() chan *api_pb.UpdateCommissionsEvent {
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

func (s *Service) SetChasingMode(val bool) {
	s.chasingMode.Store(val)
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
	chasingMode, ok := s.chasingMode.Load().(bool)
	if !ok {
		s.logger.Error("chasing mode setup error")
		return
	}
	if chasingMode {
		return
	}
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
		res := new(BalanceResource).Transform(mBalance)
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
	chasingMode, ok := s.chasingMode.Load().(bool)
	if !ok {
		s.logger.Error("chasing mode setup error")
		return
	}
	if chasingMode {
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
func (s *Service) PublishCommissions(data *api_pb.UpdateCommissionsEvent) {
	channel := `commissions`
	msg, err := protojson.Marshal(data)
	if err != nil {
		s.logger.Error(err)
	}
	s.publish(channel, msg)
}

func (s *Service) PublishStake(tx *api_pb.TransactionResponse) {
	chasingMode, ok := s.chasingMode.Load().(bool)
	if !ok {
		s.logger.Error("chasing mode setup error")
		return
	}
	if chasingMode {
		return
	}

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
	resp, err := s.client.Publish(context.Background(), &centrifugopb.PublishRequest{
		Channel: ch,
		Data:    msg,
	})
	if err != nil {
		s.logger.Errorf("Transport level error: %v", err)
	} else {
		if resp.GetError() != nil {
			respError := resp.GetError()
			s.logger.Errorf("Error %d (%s)", respError.Code, respError.Message)
		}
	}

	//_, err := s.client.Publish(context.Background(), ch, msg)
	//if err != nil {
	//	s.logger.Warn(err)
	//}
}
