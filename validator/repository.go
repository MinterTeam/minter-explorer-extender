package validator

import (
	"fmt"
	"github.com/MinterTeam/minter-explorer-tools/v4/models"
	"github.com/go-pg/pg"
	"os"
	"sync"
)

type Repository struct {
	db    *pg.DB
	cache *sync.Map
}

func NewRepository() *Repository {
	//Init DB
	db := pg.Connect(&pg.Options{
		Addr:     fmt.Sprintf("%s:%s", os.Getenv("DB_HOST"), os.Getenv("DB_PORT")),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		Database: os.Getenv("DB_NAME"),
	})

	return &Repository{
		db:    db,
		cache: new(sync.Map), //TODO: добавить реализацию очистки
	}
}

//Find validator with public key.
//Return Validator ID
func (r *Repository) FindIdByPk(pk string) (uint64, error) {
	//First look in the cache
	id, ok := r.cache.Load(pk)
	if ok {
		return id.(uint64), nil
	}
	validator := new(models.Validator)
	err := r.db.Model(validator).Column("id").Where("public_key = ?", pk).Select()
	if err != nil {
		return 0, err
	}
	r.cache.Store(pk, validator.ID)
	return validator.ID, nil
}

//Find validator with public key or create if not exist.
//Return Validator ID
func (r *Repository) FindIdByPkOrCreate(pk string) (uint64, error) {
	id, _ := r.FindIdByPk(pk)
	if id == 0 {
		validator := &models.Validator{PublicKey: pk}
		err := r.db.Insert(validator)
		if err != nil {
			return 0, err
		}
		r.cache.Store(validator.PublicKey, validator.ID)
		return validator.ID, nil
	}
	return id, nil
}

// Save list of validators if not exist
func (r *Repository) SaveAllIfNotExist(validators []*models.Validator) error {
	if r.isAllAddressesInCache(validators) {
		return nil
	}
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
	if err != nil {
		return nil, err
	}
	r.addToCache(vList)
	return vList, err
}

func (r *Repository) UpdateAll(validators []*models.Validator) error {
	_, err := r.db.Model(&validators).
		Column("status").
		Column("commission").
		Column("reward_address_id").
		Column("owner_address_id").
		Column("total_stake").
		WherePK().
		Update()
	return err
}

func (r *Repository) Update(validator *models.Validator) error {
	return r.db.Update(validator)
}

func (r Repository) DeleteStakesNotInListIds(idList []uint64) error {
	if len(idList) > 0 {
		_, err := r.db.Query(nil, `delete from stakes where id not in (?);`, pg.In(idList))
		return err

	}
	return nil
}

func (r Repository) DeleteStakesByValidatorIds(idList []uint64) error {
	if len(idList) > 0 {
		_, err := r.db.Query(nil, `delete from stakes where validator_id in (?);`, pg.In(idList))
		return err

	}
	return nil
}

func (r *Repository) SaveAllStakes(stakes []*models.Stake) error {
	_, err := r.db.Model(&stakes).OnConflict("(owner_address_id, validator_id, coin_id) DO UPDATE").Insert()
	return err
}

func (r *Repository) addToCache(validators []*models.Validator) {
	for _, v := range validators {
		_, exist := r.cache.Load(v.PublicKey)
		if !exist {
			r.cache.Store(v.PublicKey, v.ID)
		}
	}
}

func (r *Repository) isAllAddressesInCache(validators []*models.Validator) bool {
	// look PK in cache
	for _, v := range validators {
		_, exist := r.cache.Load(v.PublicKey)
		if !exist {
			return false
		}
	}
	return true
}

func (r Repository) ResetAllStatuses() error {
	_, err := r.db.Query(nil, `update validators set status = null;`)
	return err
}
