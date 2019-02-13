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
		cache: new(sync.Map), //TODO: добавить реализацию очистки
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
	}

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

// Save list of validators if not exist
func (r *Repository) SaveAllIfNotExist(validators []*models.Validator) error {
	var args []interface{}

	// Search in DB (use for update cache)
	_, _ = r.FindAllByPK(validators)

	// look PK in cache
	for _, v := range validators {
		_, exist := r.cache.Load(v.PublicKey)
		if !exist {
			args = append(args, v)
		}
	}
	// if all PK exists in cache do nothing
	if len(args) == 0 {
		return nil
	}
	err := r.db.Insert(args...)
	if err != nil {
		return err
	}
	r.addToCache(validators)
	return nil
}

// Find validators by PK
// Update cache
// Return slice of validators
func (r *Repository) FindAllByPK(validators []*models.Validator) ([]*models.Validator, error) {
	var pkList []string
	var vList []*models.Validator
	for _, v := range validators {
		pkList = append(pkList, v.PublicKey)
	}
	err := r.db.Model(&vList).Where("public_key in (?)", pg.In(pkList)).Select()
	r.addToCache(vList)
	return vList, err
}

func (r *Repository) addToCache(validators []*models.Validator) {
	for _, v := range validators {
		_, exist := r.cache.Load(v.PublicKey)
		if !exist {
			r.cache.Store(v.PublicKey, v.ID)
		}
	}
}
