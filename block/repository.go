package block

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

func (r *Repository) Save(block *models.Block) error {
	_, err := r.db.Model(block).Insert()
	if err != nil {
		return err
	}
	return nil
}

func (r *Repository) GetLastFromDB() (*models.Block, error) {
	block := new(models.Block)
	err := r.db.Model(block).Last()
	if err != nil {
		return nil, err
	}
	return block, nil
}

func (r *Repository) LinkWithValidators(links []*models.BlockValidator) error {
	var args []interface{}
	for _, l := range links {
		args = append(args, l)
	}
	err := r.db.Insert(args...)
	return err
}
