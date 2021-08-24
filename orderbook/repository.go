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

func (r *Repository) DeleteByIdList(forDelete []uint64) error {
	_, err := r.db.Model((*models.Order)(nil)).
		Where("id IN (?)", pg.In(forDelete)).
		Delete()
	return err
}

func NewRepository(db *pg.DB) *Repository {
	return &Repository{
		db: db,
	}
}
