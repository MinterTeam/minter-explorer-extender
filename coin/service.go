package coin

import (
	"encoding/json"
	"errors"
	"github.com/MinterTeam/minter-explorer-extender/address"
	"github.com/MinterTeam/minter-explorer-extender/models"
	"github.com/daniildulin/minter-node-api/responses"
	"strconv"
)

type Service struct {
	repository        *Repository
	addressRepository *address.Repository
}

func NewService(repository *Repository, addressRepository *address.Repository) *Service {
	return &Service{
		repository:        repository,
		addressRepository: addressRepository,
	}
}

type CreateCoinData struct {
	Name           string `json:"name"`
	Symbol         string `json:"symbol"`
	InitialAmount  string `json:"initial_amount"`
	InitialReserve string `json:"initial_reserve"`
	Crr            string `json:"crr"`
}

func (s Service) CreateFromTx(tx responses.Transaction) error {
	if tx.Data == nil {
		return errors.New("no data for creating a coin")
	}

	// TODO: refactor
	var data CreateCoinData
	jsonData, err := json.Marshal(*tx.Data)
	if err != nil {
		return err
	}
	err = json.Unmarshal(jsonData, &data)
	if err != nil {
		return err
	}

	txFrom := []rune(tx.From)
	fromId, err := s.addressRepository.FindIdOrCreate(string(txFrom[2:]))
	if err != nil {
		return err
	}
	crr, err := strconv.ParseUint(data.Crr, 10, 64)
	if err != nil {
		return err
	}
	return s.repository.Create(&models.Coin{
		CreationAddressID: fromId,
		Crr:               crr,
		Volume:            data.InitialAmount,
		ReserveBalance:    data.InitialReserve,
		Name:              data.Name,
		Symbol:            data.Symbol,
	})
}
