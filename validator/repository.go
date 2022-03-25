package validator

import (
	"errors"
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/go-pg/pg/v10"
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
	_, err := r.db.Model(unbond).Insert()
	return err
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
	_, err := r.db.Model(vpk).Insert()
	if err != nil {
		return err
	}
	r.cache.Store(vpk.Key, vpk.ValidatorId)
	r.pkCache.Store(vpk.Key, vpk.ID)
	return err
}

// FindIdByPk Find validator with public key.
//Return Validator ID
func (r *Repository) FindIdByPk(pk string) (uint, error) {
	//First look in the cache
	id, ok := r.cache.Load(pk)
	if ok {
		return id.(uint), nil
	}

	valId := uint(0)
	validator := new(models.Validator)
	err := r.db.Model(validator).Where("public_key = ?", pk).Select()
	if err != nil {
		validatorPk := new(models.ValidatorPublicKeys)
		err = r.db.Model(validatorPk).Where("key = ?", pk).Select()
		if err != nil {
			return 0, err
		}
		valId = validatorPk.ValidatorId
	} else {
		valId = validator.ID
	}

	if valId == 0 {
		return 0, errors.New("validator not found")
	}

	r.cache.Store(pk, valId)
	return valId, nil
}

// FindIdByPkOrCreate Find validator with public key or create if not exist.
//Return Validator ID
func (r *Repository) FindIdByPkOrCreate(pk string) (uint, error) {
	id, err := r.FindIdByPk(pk)
	if err != nil && err != pg.ErrNoRows {
		return 0, err
	}
	if id == 0 {
		validator := &models.Validator{
			PublicKey: pk,
		}
		_, err := r.db.Model(validator).Insert()
		if err != nil {
			r.log.WithField("pk", pk).Error(err)
			return 0, err
		}

		vpk := &models.ValidatorPublicKeys{
			ValidatorId: validator.ID,
			Key:         pk,
		}
		_, err = r.db.Model(vpk).Insert()
		if err != nil {
			r.log.WithField("pk", pk).Error(err)
			return 0, err
		}

		r.cache.Store(vpk.Key, validator.ID)
		return validator.ID, nil
	}
	return id, nil
}

// SaveAllIfNotExist Save list of validators if not exist
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
	for _, v := range validators {
		_, err := r.db.Model(v).
			WherePK().
			Update()

		if err != nil {
			r.log.WithField("validator", v).Error(err)
		}
	}

	return nil
}

func (r *Repository) Update(validator *models.Validator) error {
	_, err := r.db.Model(validator).WherePK().Update()
	return err
}

func (r *Repository) DeleteStakesNotInListIds(idList []uint64) error {
	if len(idList) > 0 {
		_, err := r.db.Query(nil, `delete from stakes where id not in (?) and is_kicked != true;`, pg.In(idList))
		return err
	}
	return nil
}

func (r *Repository) DeleteStakesByValidatorIds(idList []uint64) error {
	if len(idList) > 0 {
		_, err := r.db.Query(nil, `delete from stakes where validator_id in (?) and is_kicked != true;`, pg.In(idList))
		return err
	}
	return nil
}

func (r *Repository) SaveAllStakes(stakes []*models.Stake) error {
	_, err := r.db.Model(&stakes).OnConflict("(owner_address_id, validator_id, coin_id, is_kicked) DO UPDATE").Insert()
	return err
}

func (r *Repository) ResetAllStatuses() error {
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
	_, err := r.db.Model(s).OnConflict("(owner_address_id, coin_id, validator_id, is_kicked) DO UPDATE").Insert()
	return err
}
func (r *Repository) UpdateStakes(list []*models.Stake) error {
	_, err := r.db.Model(&list).OnConflict("(owner_address_id, coin_id, validator_id, is_kicked) DO UPDATE").Insert()
	return err
}

func (r *Repository) DeleteFromWaitList(addressId, validatorId uint, coins []uint64) error {
	_, err := r.db.Model().Exec(`
		DELETE FROM stakes
		WHERE owner_address_id = ? AND validator_id = ? AND is_kicked = true AND coin_id NOT IN (?);
	`, addressId, validatorId, pg.In(coins))
	return err
}

func (r *Repository) RemoveFromWaitList(addressId, validatorId uint) error {
	_, err := r.db.Model().Exec(`
		DELETE FROM stakes
		WHERE owner_address_id = ? AND validator_id = ? AND is_kicked = true ;
	`, addressId, validatorId)
	return err
}

func (r *Repository) SaveBan(ban *models.ValidatorBan) error {
	_, err := r.db.Model(ban).Insert()
	return err
}

func (r *Repository) GetStake(addressId, validatorId, coinId uint64) (*models.Stake, error) {
	stk := new(models.Stake)

	err := r.db.Model(stk).
		Where("owner_address_id = ?", addressId).
		Where("validator_id = ?", validatorId).
		Where("coin_id = ?", coinId).
		Select()

	return stk, err
}

func (r *Repository) MoveStake(ms *models.MovedStake) error {
	_, err := r.db.Model(ms).Insert()
	return err
}

func (r *Repository) DeleteOldUnbonds(height uint64) interface{} {
	_, err := r.db.Model().Exec(`
		DELETE FROM unbonds
		WHERE block_id < ?;
	`, height)
	return err
}

func (r *Repository) DeleteOldMovedStakes(height uint64) interface{} {
	_, err := r.db.Model().Exec(`
		DELETE FROM moved_stakes
		WHERE block_id < ?;
	`, height)
	return err
}

func (r *Repository) DeleteStake(addressId, validatorId, coinId uint64) error {
	_, err := r.db.Model().Exec(`
		DELETE FROM stakes
		WHERE owner_address_id = ? AND validator_id = ? AND coin_id = ? ;
	`, addressId, validatorId, coinId)
	return err
}
