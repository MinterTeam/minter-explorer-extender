package address

import (
	"github.com/MinterTeam/minter-explorer-extender/models"
	"github.com/go-pg/pg"
	"log"
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
func (r *Repository) Save(block *models.Block) (*models.Block, error) {
	_, err := r.db.Model(block).Insert()
	if err != nil {
		log.Println(err)
	}
	return block, nil
}
