package orderbook

import (
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/go-pg/pg/v10"
)

type Repository struct {
	db *pg.DB
}

func (r *Repository) SaveAll(list []*models.Order) error {
	_, err := r.db.Model(&list).Insert()
	return err
}

func (r *Repository) CancelByIdList(forCancel []uint64, cancelType models.OrderType) error {
	_, err := r.db.Model().Exec(`
		UPDATE orders SET status = ?
		WHERE id NOT IN (?);
	`, cancelType, pg.In(forCancel))
	return err
}

func (r *Repository) GetAllById(id []uint64) ([]models.Order, error) {
	var list []models.Order
	err := r.db.Model(&list).Where("id in (?)", pg.In(id)).Select()
	return list, err
}

func (r *Repository) UpdateOrders(list *[]models.Order) error {
	_, err := r.db.Model(&list).WherePK().Update()
	return err
}

func NewRepository(db *pg.DB) *Repository {
	return &Repository{
		db: db,
	}
}
