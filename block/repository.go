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

//Find address id or create if not exist
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
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	// Rollback tx on error.
	defer tx.Rollback()
	for _, v := range links {
		_, err = tx.Model(v).OnConflict(`DO NOTHING`).Insert()
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
