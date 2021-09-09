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

func NewRepository(db *pg.DB) *Repository {
	return &Repository{
		db: db,
	}
}
