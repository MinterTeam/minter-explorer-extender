package balance

import (
	"github.com/MinterTeam/minter-explorer-tools/models"
	"github.com/go-pg/pg"
)

type Repository struct {
	db *pg.DB
}

func NewRepository(db *pg.DB) *Repository {
	return &Repository{
		db: db,
	}
}

func (r *Repository) FindAllByAddress(addresses []string) ([]*models.Balance, error) {
	var balances []*models.Balance
	err := r.db.Model(&balances).
		Column("balance.*", "Address", "Coin").
		Where("address.address in (?)", pg.In(addresses)).
		Select()
	return balances, err
}

func (r *Repository) SaveAll(balances []*models.Balance) error {
	var args []interface{}
	for _, balance := range balances {
		args = append(args, balance)
	}
	// if all addresses do nothing
	if len(args) == 0 {
		return nil
	}
	return r.db.Insert(args...)
}

func (r *Repository) UpdateAll(balances []*models.Balance) error {
	_, err := r.db.Model(&balances).Update()
	return err
}

func (r *Repository) DeleteAll(balances []*models.Balance) error {
	_, err := r.db.Model(&balances).Delete()
	return err
}

func (r Repository) DeleteByCoinId(coinId uint64) error {
	_, err := r.db.Model(new(models.Balance)).Where("coin_id = ?", coinId).Delete()
	return err
}
