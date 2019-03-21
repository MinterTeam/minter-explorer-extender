package validator

import (
	"github.com/MinterTeam/minter-explorer-tools/models"
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

func (r *Repository) Update(validator *models.Validator) error {
	return r.db.Update(validator)
}

func (r *Repository) UpdateStakesByValidatorId(validatorId uint64, stakes []*models.Stake) error {
	// Delete all validators stakes before
	_, err := r.db.Model(new(models.Stake)).Where("validator_id = ?", validatorId).Delete()
	if err != nil {
		return err
	}
	var args []interface{}
	for _, stake := range stakes {
		args = append(args, stake)
	}
	// if all addresses do nothing
	if len(args) == 0 {
		return nil
	}
	return r.db.Insert(args...)
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
	_, err := r.db.Query(nil, `update validators set status = null`)
	return err
}
