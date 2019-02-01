package validator

import (
	"github.com/MinterTeam/minter-explorer-extender/models"
	"github.com/go-pg/pg"
	"sync"
)

type Repository struct {
	db    *pg.DB
	cache *sync.Map
}

func NewRepository(db *pg.DB) *Repository {
	return &Repository{
		db:    db,
		cache: new(sync.Map),
	}
}

//Find validator with public key.
//Create if not exist
//Return Validator ID
func (r *Repository) FindIdOrCreateByPk(pk string) (uint64, error) {
	//First look in the cache
	id, ok := r.cache.Load(pk)
	if ok {
		return id.(uint64), nil
	} else {
		validator := models.Validator{PublicKey: pk}
		_, err := r.db.Model(&validator).
			Where("public_key = ?public_key").
			OnConflict("DO NOTHING").
			SelectOrInsert()
		if err != nil {
			return 0, err
		}
		r.cache.Store(pk, validator.ID)
		return validator.ID, nil
	}
}

//Save list of validators
func (r *Repository) MassSave(validators []*models.Validator) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	// Rollback tx on error.
	defer tx.Rollback()
	for _, v := range validators {
		_, err = tx.Model(v).OnConflict(`DO NOTHING`).Insert()
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
