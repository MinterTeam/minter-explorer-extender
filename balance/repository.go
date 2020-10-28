package balance

import (
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/go-pg/pg/v10"
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
		Column("balance.*").
		Relation("Address").
		Relation("Coin").
		Where("address.address in (?)", pg.In(addresses)).
		Select()
	return balances, err
}

func (r *Repository) SaveAll(balances []*models.Balance) error {
	_, err := r.db.Model(&balances).OnConflict("(address_id, coin_id) DO UPDATE").Insert()
	return err
}

func (r *Repository) UpdateAll(balances []*models.Balance) error {
	_, err := r.db.Model(&balances).Update()
	return err
}

func (r *Repository) DeleteAll(balances []*models.Balance) error {
	_, err := r.db.Model(&balances).Delete()
	return err
}

func (r *Repository) DeleteByCoinId(coinId uint) error {
	_, err := r.db.Model(new(models.Balance)).Where("id = ?", coinId).Delete()
	return err
}
