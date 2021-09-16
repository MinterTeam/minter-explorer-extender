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
	var idsCanceled []uint64
	var idsPartiallyFilled []uint64
	for _, o := range list {
		switch o.Status {
		case models.OrderTypeNew:
			idsCanceled = append(idsCanceled, o.Id)
		case models.OrderTypeActive:
			idsPartiallyFilled = append(idsPartiallyFilled, o.Id)
		}
	}

	_, err = r.db.Model().Exec(`
		UPDATE orders SET status = ?
		WHERE id IN (?);
	`, cancelType, pg.In(idsCanceled))

	_, err = r.db.Model().Exec(`
		UPDATE orders SET status = ?
		WHERE id IN (?);
	`, models.OrderTypePartiallyFilled, pg.In(idsPartiallyFilled))
	return err
}

func (r *Repository) GetAllById(id []uint64) ([]models.Order, error) {
	var list []models.Order
	err := r.db.Model(&list).Where("id in (?)", pg.In(id)).Select()
	return list, err
}

func (r *Repository) UpdateOrders(list *[]models.Order) error {
	_, err := r.db.Model(list).WherePK().Update()
	return err
}

func NewRepository(db *pg.DB) *Repository {
	return &Repository{
		db: db,
	}
}
