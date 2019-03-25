package address

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/MinterTeam/minter-explorer-tools/helpers"
	"github.com/MinterTeam/minter-explorer-tools/models"
	"github.com/MinterTeam/minter-go-node/core/check"
	"github.com/daniildulin/minter-node-api/responses"
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
}

func NewService(env *models.ExtenderEnvironment, repository *Repository, chBalanceAddresses chan<- models.BlockAddresses) *Service {
	return &Service{
		env:                env,
		repository:         repository,
		chBalanceAddresses: chBalanceAddresses,
		jobSaveAddresses:   make(chan []string, env.WrkSaveAddressesCount),
	}
}

func (s *Service) GetSaveAddressesJobChannel() chan []string {
	return s.jobSaveAddresses
}

func (s *Service) SaveAddressesWorker(jobs <-chan []string) {
	for addresses := range jobs {
		err := s.repository.SaveAllIfNotExist(addresses)
		helpers.HandleError(err)
		s.wgAddresses.Done()
	}
}

func (s *Service) ExtractAddressesFromTransactions(transactions []responses.Transaction) ([]string, error, map[string]struct{}) {
	var mapAddresses = make(map[string]struct{}) //use as unique array
	for _, tx := range transactions {
		if tx.Data == nil {
			return nil, errors.New("empty transaction data"), nil
		}
		mapAddresses[helpers.RemovePrefix(tx.From)] = struct{}{}
		if tx.Type == models.TxTypeSend {
			var txData models.SendTxData
			jsonData, err := json.Marshal(*tx.Data)
			if err != nil {
				return nil, err, nil
			}
			err = json.Unmarshal(jsonData, &txData)
			if err != nil {
				return nil, err, nil
			}
			mapAddresses[helpers.RemovePrefix(txData.To)] = struct{}{}
		}
		if tx.Type == models.TxTypeMultiSend {
			var txData models.MultiSendTxData
			jsonData, err := json.Marshal(*tx.Data)
			if err != nil {
				return nil, err, nil
			}
			err = json.Unmarshal(jsonData, &txData)
			if err != nil {
				return nil, err, nil
			}
			for _, receiver := range txData.List {
				mapAddresses[helpers.RemovePrefix(receiver.To)] = struct{}{}
			}
		}
		if tx.Type == models.TxTypeRedeemCheck {
			var txData models.RedeemCheckTxData
			decoded, err := base64.StdEncoding.DecodeString(txData.RawCheck)
			if err != nil {
				return nil, err, nil
			}
			data, err := check.DecodeFromBytes(decoded)
			if err != nil {
				return nil, err, nil
			}
			sender, err := data.Sender()
			if err != nil {
				return nil, err, nil
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
		mapAddresses[helpers.RemovePrefix(event.Value.Address)] = struct{}{}
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

	if blockResponse != nil && blockResponse.Result.TxCount != "0" {
		height, err = strconv.ParseUint(blockResponse.Result.Height, 10, 64)
		if err != nil {
			return err
		}
		_, err, blockAddressesMap = s.ExtractAddressesFromTransactions(blockResponse.Result.Transactions)
		if err != nil {
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
