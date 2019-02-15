package address

import (
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
		data := *tx.Data
		txFrom := []rune(tx.From)
		mapAddresses[string(txFrom[2:])] = struct{}{}
		if tx.Type == models.TxTypeSend {
			to := []rune(data["to"].(string))
			if len(to) > 0 {
				mapAddresses[string(to[2:])] = struct{}{}
			}
		}
		if tx.Type == models.TxTypeMultiSend {
			list := data["list"].([]interface{})
			for _, item := range list {
				receiver := item.(map[string]interface{})
				to := []rune(receiver["to"].(string))
				if len(to) > 0 {
					mapAddresses[string(to[2:])] = struct{}{}
				}
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
