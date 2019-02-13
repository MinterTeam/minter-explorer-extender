package coin

import (
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

func (s Service) CreateFromTx(tx responses.Transaction) error {
	if tx.Data == nil {
		return errors.New("no data for creating a coin")
	}
	data := *tx.Data
	name := data["name"].(string)
	symbol := data["symbol"].(string)
	initialAmount := data["initial_amount"].(string)
	initialReserve := data["initial_reserve"].(string)
	txFrom := []rune(tx.From)
	fromId, err := s.addressRepository.FindIdOrCreate(string(txFrom[2:]))
	crr, err := strconv.ParseUint(data["constant_reserve_ratio"].(string), 10, 64)
	if err != nil {
		return err
	}
	return s.repository.Create(&models.Coin{
		CreationAddressID: fromId,
		Crr:               crr,
		Volume:            initialAmount,
		ReserveBalance:    initialReserve,
		Name:              name,
		Symbol:            symbol,
	})
}
