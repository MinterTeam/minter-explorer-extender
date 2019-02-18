package address

import (
	"encoding/json"
	"errors"
	"github.com/MinterTeam/minter-explorer-extender/helpers"
	"github.com/MinterTeam/minter-explorer-extender/models"
	"github.com/daniildulin/minter-node-api/responses"
)

type Service struct {
	repository *Repository
}

func NewService(repository *Repository) *Service {
	return &Service{
		repository: repository,
	}
}

// Find all addresses in block response and save it
func (s *Service) HandleTransactionsFromBlockResponse(transactions []responses.Transaction) error {
	var mapAddresses = make(map[string]struct{}) //use as unique array
	for _, tx := range transactions {
		if tx.Data == nil {
			return errors.New("empty transaction data")
		}
		mapAddresses[helpers.RemovePrefix(tx.From)] = struct{}{}
		if tx.Type == models.TxTypeSend {
			var txData models.SendTxData
			jsonData, err := json.Marshal(*tx.Data)
			if err != nil {
				return err
			}
			err = json.Unmarshal(jsonData, &txData)
			if err != nil {
				return err
			}
			mapAddresses[helpers.RemovePrefix(txData.To)] = struct{}{}
		}
		if tx.Type == models.TxTypeMultiSend {
			var txData models.MultiSendTxData
			jsonData, err := json.Marshal(*tx.Data)
			if err != nil {
				return err
			}
			err = json.Unmarshal(jsonData, &txData)
			if err != nil {
				return err
			}
			for _, receiver := range txData.List {
				mapAddresses[helpers.RemovePrefix(receiver.To)] = struct{}{}
			}
		}
	}
	addresses := make([]string, len(mapAddresses))
	i := 0
	for a := range mapAddresses {
		addresses[i] = a
		i++
	}
	return s.repository.SaveAllIfNotExist(addresses)
}

func (s *Service) HandleEventsResponse(response *responses.EventsResponse) error {
	var mapAddresses = make(map[string]struct{}) //use as unique array
	for _, event := range response.Result.Events {
		mapAddresses[helpers.RemovePrefix(event.Value.Address)] = struct{}{}
	}
	addresses := make([]string, len(mapAddresses))
	i := 0
	for a := range mapAddresses {
		addresses[i] = a
		i++
	}
	return s.repository.SaveAllIfNotExist(addresses)
}
