package address

import (
	"github.com/MinterTeam/minter-explorer-extender/v2/env"
	"github.com/MinterTeam/minter-explorer-tools/v4/helpers"
	"github.com/MinterTeam/minter-go-sdk/v2/api"
	"github.com/MinterTeam/minter-go-sdk/v2/api/grpc_client"
	"github.com/MinterTeam/minter-go-sdk/v2/transaction"
	"github.com/MinterTeam/node-grpc-gateway/api_pb"
	"github.com/sirupsen/logrus"
	"math"
	"sync"
)

type Service struct {
	env              *env.ExtenderEnvironment
	Storage          *Repository
	jobSaveAddresses chan []string
	wgAddresses      sync.WaitGroup
	logger           *logrus.Entry
}

func NewService(env *env.ExtenderEnvironment, repository *Repository, logger *logrus.Entry) *Service {
	return &Service{
		env:              env,
		Storage:          repository,
		jobSaveAddresses: make(chan []string, env.WrkSaveAddressesCount),
		logger:           logger,
	}
}

func (s *Service) GetSaveAddressesJobChannel() chan []string {
	return s.jobSaveAddresses
}

func (s *Service) SaveAddressesWorker(jobs <-chan []string) {
	for addresses := range jobs {
		err := s.Storage.SaveAllIfNotExist(addresses)
		s.wgAddresses.Done()
		if err != nil {
			s.logger.Panic(err)
		}
	}
}

func (s *Service) ExtractAddressesFromTransactions(transactions []*api_pb.TransactionResponse) ([]string, error, map[string]struct{}) {
	var mapAddresses = make(map[string]struct{}) //use as unique array
	for _, tx := range transactions {
		mapAddresses[helpers.RemovePrefix(tx.From)] = struct{}{}

		switch transaction.Type(tx.Type) {
		case transaction.TypeSend:
			txData := new(api_pb.SendData)
			if err := tx.Data.UnmarshalTo(txData); err != nil {
				return nil, err, nil
			}
			mapAddresses[helpers.RemovePrefix(txData.To)] = struct{}{}
		case transaction.TypeMultisend:
			txData := new(api_pb.MultiSendData)
			if err := tx.Data.UnmarshalTo(txData); err != nil {
				return nil, err, nil
			}
			for _, receiver := range txData.List {
				mapAddresses[helpers.RemovePrefix(receiver.To)] = struct{}{}
			}
		case transaction.TypeCreateMultisig:
			txData := new(api_pb.CreateMultisigData)
			if err := tx.Data.UnmarshalTo(txData); err != nil {
				return nil, err, nil
			}
			for _, adr := range txData.Addresses {
				mapAddresses[helpers.RemovePrefix(adr)] = struct{}{}
			}
		case transaction.TypeEditMultisig:
			txData := new(api_pb.EditMultisigData)
			if err := tx.Data.UnmarshalTo(txData); err != nil {
				return nil, err, nil
			}
			for _, adr := range txData.Addresses {
				mapAddresses[helpers.RemovePrefix(adr)] = struct{}{}
			}
		case transaction.TypeRedeemCheck:
			txData := new(api_pb.RedeemCheckData)
			if err := tx.Data.UnmarshalTo(txData); err != nil {
				return nil, err, nil
			}

			data, err := transaction.DecodeCheckBase64(txData.RawCheck)
			if err != nil {
				s.logger.WithFields(logrus.Fields{
					"RawCheck": txData.RawCheck,
					"Tx":       tx.Hash,
				}).Error(err)
				return nil, err, nil
			}
			sender, err := data.Sender()
			if err != nil {
				s.logger.Error(err)
				return nil, err, nil
			}
			mapAddresses[helpers.RemovePrefix(sender)] = struct{}{}
		}
	}
	addresses := addressesMapToSlice(mapAddresses)
	return addresses, nil, mapAddresses
}

func (s *Service) ExtractAddressesEventsResponse(response *api_pb.EventsResponse) ([]string, map[string]struct{}) {
	var mapAddresses = make(map[string]struct{}) //use as unique array
	for _, event := range response.Events {
		eventStruct, err := grpc_client.ConvertStructToEvent(event)
		if err != nil {
			return nil, mapAddresses
		}

		if stake, ok := eventStruct.(api.StakeEvent); ok {
			mapAddresses[helpers.RemovePrefix(stake.GetAddress())] = struct{}{}
		}
	}
	addresses := addressesMapToSlice(mapAddresses)
	return addresses, mapAddresses
}

// SaveAddressesFromResponses Find all addresses in block response and save it
func (s *Service) SaveAddressesFromResponses(blockResponse *api_pb.BlockResponse, eventsResponse *api_pb.EventsResponse) error {
	var (
		err                error
		blockAddressesMap  = make(map[string]struct{})
		eventsAddressesMap = make(map[string]struct{})
	)

	if blockResponse != nil && len(blockResponse.Transactions) > 0 {
		_, err, blockAddressesMap = s.ExtractAddressesFromTransactions(blockResponse.Transactions)
		if err != nil {
			return err
		}
	}
	if eventsResponse != nil && len(eventsResponse.Events) > 0 {
		_, eventsAddressesMap = s.ExtractAddressesEventsResponse(eventsResponse)
	}
	// Merge maps
	for k, v := range eventsAddressesMap {
		blockAddressesMap[k] = v
	}

	addresses := addressesMapToSlice(blockAddressesMap)

	if len(addresses) > 0 {
		chunksCount := int(math.Ceil(float64(len(addresses)) / float64(s.env.TxChunkSize)))
		s.wgAddresses.Add(chunksCount)
		for i := 0; i < chunksCount; i++ {
			start := s.env.TxChunkSize * i
			end := start + s.env.TxChunkSize
			if end > len(addresses) {
				end = len(addresses)
			}
			s.GetSaveAddressesJobChannel() <- addresses[start:end]
		}
		s.wgAddresses.Wait()
	}
	return nil
}

func addressesMapToSlice(mapAddresses map[string]struct{}) []string {
	addresses := make([]string, len(mapAddresses))
	i := 0
	for a := range mapAddresses {
		if len(a) > 40 {
			addresses[i] = helpers.RemovePrefix(a)
		} else {
			addresses[i] = a
		}
		i++
	}
	return addresses
}
