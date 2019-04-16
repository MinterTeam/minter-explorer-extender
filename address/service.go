package address

import (
	"encoding/base64"
	"errors"
	"github.com/MinterTeam/minter-explorer-tools/helpers"
	"github.com/MinterTeam/minter-explorer-tools/models"
	"github.com/MinterTeam/minter-go-node/core/check"
	"github.com/MinterTeam/minter-node-go-api/responses"
	"github.com/sirupsen/logrus"
	"math"
	"strconv"
	"sync"
)

type Service struct {
	env                *models.ExtenderEnvironment
	repository         *Repository
	chBalanceAddresses chan<- models.BlockAddresses
	jobSaveAddresses   chan []string
	wgAddresses        sync.WaitGroup
	logger             *logrus.Entry
}

func NewService(env *models.ExtenderEnvironment, repository *Repository, chBalanceAddresses chan<- models.BlockAddresses, logger *logrus.Entry) *Service {
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

func (s *Service) ExtractAddressesFromTransactions(transactions []responses.Transaction) ([]string, error, map[string]struct{}) {
	var mapAddresses = make(map[string]struct{}) //use as unique array
	for _, tx := range transactions {
		if tx.Data == nil {
			s.logger.Error("empty transaction data")
			return nil, errors.New("empty transaction data"), nil
		}
		mapAddresses[helpers.RemovePrefix(tx.From)] = struct{}{}
		if tx.Type == models.TxTypeSend {
			mapAddresses[helpers.RemovePrefix(tx.RawData.(models.SendTxData).To)] = struct{}{}
		}
		if tx.Type == models.TxTypeMultiSend {
			for _, receiver := range tx.RawData.(models.MultiSendTxData).List {
				mapAddresses[helpers.RemovePrefix(receiver.To)] = struct{}{}
			}
		}
		if tx.Type == models.TxTypeRedeemCheck {
			decoded, err := base64.StdEncoding.DecodeString(tx.RawData.(models.RedeemCheckTxData).RawCheck)
			if err != nil {
				s.logger.WithFields(logrus.Fields{
					"Tx": tx.Hash,
				}).Error(err)
				continue
			}
			data, err := check.DecodeFromBytes(decoded)
			if err != nil {
				s.logger.WithFields(logrus.Fields{
					"Tx": tx.Hash,
				}).Error(err)
				continue
			}
			sender, err := data.Sender()
			if err != nil {
				s.logger.WithFields(logrus.Fields{
					"Tx": tx.Hash,
				}).Error(err)
				continue
			}
			mapAddresses[helpers.RemovePrefix(sender.String())] = struct{}{}
		}
	}
	addresses := addressesMapToSlice(mapAddresses)
	return addresses, nil, mapAddresses
}

func (s *Service) ExtractAddressesEventsResponse(response *responses.EventsResponse) ([]string, map[string]struct{}) {
	var mapAddresses = make(map[string]struct{}) //use as unique array
	for _, event := range response.Result.Events {

		addressesHash := event.Value.Address

		if len(addressesHash) > 2 {
			addressesHash = helpers.RemovePrefix(addressesHash)
			mapAddresses[addressesHash] = struct{}{}
		}
	}
	addresses := addressesMapToSlice(mapAddresses)

	return addresses, mapAddresses
}

// Find all addresses in block response and save it
func (s *Service) HandleResponses(blockResponse *responses.BlockResponse, eventsResponse *responses.EventsResponse) error {
	var (
		err                error
		height             uint64
		blockAddressesMap  = make(map[string]struct{})
		eventsAddressesMap = make(map[string]struct{})
	)

	if blockResponse != nil {
		height, err = strconv.ParseUint(blockResponse.Result.Height, 10, 64)
		if err != nil {
			s.logger.Error(err)
			return err
		}
	}
	if blockResponse != nil && blockResponse.Result.TxCount != "0" {
		_, err, blockAddressesMap = s.ExtractAddressesFromTransactions(blockResponse.Result.Transactions)
		if err != nil {
			s.logger.Error(err)
			return err
		}
	}
	if eventsResponse != nil && len(eventsResponse.Result.Events) > 0 {
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
