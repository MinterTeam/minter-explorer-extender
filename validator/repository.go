package validator

import (
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/go-pg/pg/v9"
	"github.com/sirupsen/logrus"
	"sync"
)

type Repository struct {
	db      *pg.DB
	cache   *sync.Map
	pkCache *sync.Map
	log     *logrus.Entry
}

func NewRepository(db *pg.DB, logger *logrus.Entry) *Repository {
	return &Repository{
		db:      db,
		cache:   new(sync.Map), //TODO: добавить реализацию очистки
		pkCache: new(sync.Map), //TODO: добавить реализацию очистки
		log: logger.WithFields(logrus.Fields{
			"service": "Validator repository",
		}),
	}
}

func (r *Repository) AddUnbond(unbond *models.Unbond) error {
	return r.db.Insert(unbond)
}

func (r *Repository) GetById(id uint) (*models.Validator, error) {
	validator := new(models.Validator)
	err := r.db.Model(validator).
		Where("id = ?", id).
		Select()

	return validator, err
}

func (r *Repository) AddPk(id uint, pk string) error {
	vpk := &models.ValidatorPublicKeys{
		ValidatorId: id,
		Key:         pk,
	}
	err := r.db.Insert(vpk)
	if err != nil {
		return err
	}
	r.cache.Store(vpk.Key, vpk.ValidatorId)
	r.pkCache.Store(vpk.Key, vpk.ID)
	return err
}

//Find validator with public key.
//Return Validator ID
func (r *Repository) FindIdByPk(pk string) (uint, error) {
	//First look in the cache
	id, ok := r.cache.Load(pk)
	if ok {
		return id.(uint), nil
	}
	validator := new(models.ValidatorPublicKeys)
	err := r.db.Model(validator).Where("key = ?", pk).Select()
	if err != nil {
		return 0, err
	}
	r.cache.Store(pk, validator.ValidatorId)
	return validator.ValidatorId, nil
}

//Find validator with public key or create if not exist.
//Return Validator ID
func (r *Repository) FindIdByPkOrCreate(pk string) (uint, error) {
	id, err := r.FindIdByPk(pk)
	if err != nil && err.Error() != "pg: no rows in result set" {
		return 0, err
	}
	if id == 0 {
		validator := &models.Validator{
			PublicKey: pk,
		}
		err := r.db.Insert(validator)
		if err != nil {
			r.log.WithField("pk", pk).Error(err)
			return 0, err
		}

		vpk := &models.ValidatorPublicKeys{
			ValidatorId: validator.ID,
			Key:         pk,
		}
		err = r.db.Insert(vpk)
		if err != nil {
			r.log.WithField("pk", pk).Error(err)
			return 0, err
		}

		r.cache.Store(vpk.Key, validator.ID)
		return validator.ID, nil
	}
	return id, nil
}

// Save list of validators if not exist
func (r *Repository) SaveAllIfNotExist(validators map[string]struct{}) error {
	for pk, _ := range validators {
		_, err := r.FindIdByPkOrCreate(pk)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) UpdateAll(validators []*models.Validator) error {
	_, err := r.db.Model(&validators).
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

func (r Repository) ResetAllStatuses() error {
	_, err := r.db.Query(nil, `update validators set status = null;`)
	return err
}

func (r *Repository) FindPkId(pk string) (uint, error) {
	//First look in the cache
	id, ok := r.pkCache.Load(pk)
	if ok {
		return id.(uint), nil
	}
	validator := new(models.ValidatorPublicKeys)
	err := r.db.Model(validator).Where("key = ?", pk).Select()
	if err != nil {
		return 0, err
	}
	r.pkCache.Store(pk, validator.ID)
	return validator.ID, nil
}

func (r *Repository) UpdateStake(s *models.Stake) error {
	_, err := r.db.Model(s).OnConflict("(owner_address_id, coin_id, validator_id) DO UPDATE").Insert()
	return err
}

func (r *Repository) DeleteFromWaitList(addressId, validatorId uint, coins []uint64) error {
	_, err := r.db.Model().Exec(`
		UPDATE stakes SET is_kicked = false
		WHERE owner_address_id = ? AND validator_id = ? AND coin_id NOT IN (?);
	`, addressId, validatorId, pg.In(coins))
	return err
}

func (r *Repository) RemoveFromWaitList(addressId, validatorId uint) error {
	_, err := r.db.Model().Exec(`
		UPDATE stakes SET is_kicked = false
		WHERE owner_address_id = ? AND validator_id = ?;
	`, addressId, validatorId)
	return err
}
