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

func (r *Repository) CancelByIdList(idList []uint64, cancelType models.OrderType) error {
	var list []models.Order
	err := r.db.Model(&list).Where("id in (?)", pg.In(idList)).Select()
	if err != nil {

	}
	var ids []uint64
	for _, o := range list {
		if o.Status != models.OrderTypeCanceled && o.Status != models.OrderTypeFilled {
			ids = append(ids, o.Id)
		}
	}

	_, err = r.db.Model().Exec(`
		UPDATE orders SET status = ?
		WHERE id IN (?);
	`, cancelType, pg.In(ids))
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
