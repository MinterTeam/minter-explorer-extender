package address

import (
	"encoding/hex"
	"errors"
	"github.com/MinterTeam/minter-explorer-extender/env"
	"github.com/MinterTeam/minter-explorer-tools/v4/helpers"
	"github.com/MinterTeam/minter-explorer-tools/v4/models"
	"github.com/MinterTeam/minter-go-sdk/api"
	"github.com/MinterTeam/minter-go-sdk/transaction"
	"github.com/sirupsen/logrus"
	"math"
	"strconv"
	"sync"
)

type Service struct {
	env                *env.ExtenderEnvironment
	repository         *Repository
	chBalanceAddresses chan<- models.BlockAddresses
	jobSaveAddresses   chan []string
	wgAddresses        sync.WaitGroup
	logger             *logrus.Entry
}

func NewService(env *env.ExtenderEnvironment, repository *Repository, chBalanceAddresses chan<- models.BlockAddresses, logger *logrus.Entry) *Service {
	return &Service{
		env:                env,
		repository:         repository,
		chBalanceAddresses: chBalanceAddresses,
		jobSaveAddresses:   make(chan []string, env.WrkSaveAddressesCount),
		logger:             logger,
	}
}

func (s *Service) GetSaveAddressesJobChannel() chan []string {
	return s.jobSaveAddresses
}

func (s *Service) SaveAddressesWorker(jobs <-chan []string) {
	for addresses := range jobs {
		err := s.repository.SaveAllIfNotExist(addresses)
		if err != nil {
			s.logger.Error(err)
		}
		helpers.HandleError(err)

		s.wgAddresses.Done()
	}
}

func (s *Service) ExtractAddressesFromTransactions(transactions []api.TransactionResult) ([]string, error, map[string]struct{}) {
	var mapAddresses = make(map[string]struct{}) //use as unique array
	for _, tx := range transactions {
		if tx.Data == nil {
			s.logger.Error("empty transaction data")
			return nil, errors.New("empty transaction data"), nil
		}
		mapAddresses[helpers.RemovePrefix(tx.From)] = struct{}{}

		if transaction.Type(tx.Type) == transaction.TypeSend {
			var txData transaction.SendData
			err := tx.Data.FillStruct(txData)
			if tx.Data == nil {
				s.logger.Error(err)
				return nil, err, nil
			}
			to := hex.EncodeToString(txData.To[:])
			mapAddresses[to] = struct{}{}
		}

		if transaction.Type(tx.Type) == transaction.TypeMultisend {
			var txData transaction.MultisendData
			err := tx.Data.FillStruct(&txData)
			if tx.Data == nil {
				s.logger.Error(err)
				return nil, err, nil
			}
			for _, receiver := range txData.List {
				to := hex.EncodeToString(receiver.To[:])
				mapAddresses[helpers.RemovePrefix(to)] = struct{}{}
			}
		}

		if transaction.Type(tx.Type) == transaction.TypeRedeemCheck {
			txData := new(api.RedeemCheckData)
			err := tx.Data.FillStruct(txData)
			if err != nil {
				s.logger.WithFields(logrus.Fields{
					"Tx": tx.Hash,
				}).Error(err)
				return nil, err, nil
			}
			data, err := transaction.DecodeIssueCheck(txData.RawCheck)
			if err != nil {
				s.logger.WithFields(logrus.Fields{
					"Tx": tx.Hash,
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

func (s *Service) ExtractAddressesEventsResponse(response *api.EventsResult) ([]string, map[string]struct{}) {
	var mapAddresses = make(map[string]struct{}) //use as unique array
	for _, event := range response.Events {
		addressesHash := event.Value["address"]
		if len(addressesHash) > 2 {
			addressesHash = helpers.RemovePrefix(addressesHash)
			mapAddresses[addressesHash] = struct{}{}
		}
	}
	addresses := addressesMapToSlice(mapAddresses)

	return addresses, mapAddresses
}

// Find all addresses in block response and save it
func (s *Service) HandleResponses(blockResponse *api.BlockResult, eventsResponse *api.EventsResult) error {
	var (
		err                error
		height             uint64
		blockAddressesMap  = make(map[string]struct{})
		eventsAddressesMap = make(map[string]struct{})
	)

	if blockResponse != nil {
		height, err = strconv.ParseUint(blockResponse.Height, 10, 64)
		if err != nil {
			s.logger.Error(err)
			return err
		}
	}
	if blockResponse != nil && blockResponse.NumTxs != "0" {
		_, err, blockAddressesMap = s.ExtractAddressesFromTransactions(blockResponse.Transactions)
		if err != nil {
			s.logger.Error(err)
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
		for i := 0; i < chunksCount; i++ {
			start := s.env.TxChunkSize * i
			end := start + s.env.TxChunkSize
			if end > len(addresses) {
				end = len(addresses)
			}
			s.wgAddresses.Add(1)
			s.GetSaveAddressesJobChannel() <- addresses[start:end]
		}
		s.wgAddresses.Wait()

		if height != 0 {
			s.chBalanceAddresses <- models.BlockAddresses{Height: height, Addresses: addresses}
		}
	}
	return nil
}

func addressesMapToSlice(mapAddresses map[string]struct{}) []string {
	addresses := make([]string, len(mapAddresses))
	i := 0
	for a := range mapAddresses {
		addresses[i] = a
		i++
	}
	return addresses
}
