package balance

import (
	"fmt"
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/go-pg/pg/v10"
	"strings"
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
	_, err := r.db.Model(&balances).Insert()
	return err
}

func (r *Repository) UpdateAll(balances []*models.Balance) error {
	_, err := r.db.Model(&balances).WherePK().Update()
	return err
}

func (r *Repository) DeleteAll(balances []*models.Balance) error {
	_, err := r.db.Model(&balances).WherePK().Delete()
	return err
}

func (r *Repository) DeleteByCoinId(coinId uint) error {
	_, err := r.db.Model(new(models.Balance)).Where("id = ?", coinId).Delete()
	return err
}

func (r *Repository) Add(balance *models.Balance) error {
	_, err := r.db.Model(&balance).OnConflict("(address_id, coin_id) DO UPDATE").Insert()
	return err
}

func (r *Repository) GetByCoinIdAndAddressId(addressID, coinID uint) (*models.Balance, error) {
	b := new(models.Balance)
	err := r.db.Model(b).Where("address_id = ? and coin_id = ?", addressID, coinID).Select()
	return b, err
}

func (r *Repository) Delete(addressID, coinID uint) error {
	b := new(models.Balance)
	_, err := r.db.Model(b).Where("address_id = ? and coin_id = ?", addressID, coinID).Delete()
	return err
}

// Exist is a map with a key is AddressId and value is a slice of coin ids
func (r *Repository) DeleteUselessCoins(exist map[uint][]uint64) error {
	var condition []string
	for addressId, coins := range exist {
		condition = append(condition, fmt.Sprintf("(address_id = %d and coin_id not in (%s))", addressId, uintJoin(coins, ",")))
	}
	query := fmt.Sprintf("DELETE from balances WHERE %s", strings.Join(condition, " or "))
	_, err := r.db.Model((*models.Balance)(nil)).Exec(query)
	return err
}

func (r *Repository) DeleteByAddressIds(addressIds []uint) error {
	_, err := r.db.Exec(`
		DELETE FROM balances WHERE address_id in (?)
	`, pg.In(addressIds))
	return err
}

func uintJoin(array []uint64, sep string) string {
	var strArray []string
	for i := range array {
		strArray = append(strArray, fmt.Sprintf("%d", array[i]))
	}
	return strings.Join(strArray, sep)
}
