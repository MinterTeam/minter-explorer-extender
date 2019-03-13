package events

import (
	"github.com/MinterTeam/minter-explorer-tools/models"
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

func (r Repository) SaveRewards(rewards []*models.Reward) error {
	var args []interface{}
	for _, reward := range rewards {
		args = append(args, reward)
	}
	return r.db.Insert(args...)
}

func (r Repository) SaveSlashes(slashes []*models.Slash) error {
	var args []interface{}
	for _, slash := range slashes {
		args = append(args, slash)
	}
	return r.db.Insert(args...)
}
