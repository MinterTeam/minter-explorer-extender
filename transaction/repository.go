package transaction

import (
	"github.com/MinterTeam/minter-explorer-extender/models"
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

func (r *Repository) Save(transaction *models.Transaction) error {
	_, err := r.db.Model(transaction).Insert()
	if err != nil {
		return err
	}
	return nil
}

func (r *Repository) SaveAll(transactions []*models.Transaction) error {
	var args []interface{}
	for _, t := range transactions {
		args = append(args, t)
	}
	return r.db.Insert(args...)
}

func (r *Repository) SaveAllTxOutput(output []*models.TransactionOutput) error {
	var args []interface{}
	for _, t := range output {
		args = append(args, t)
	}
	return r.db.Insert(args...)
}
