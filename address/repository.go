package address

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

//Find address id or create if not exist
func (r *Repository) FindIdOrCreate(address string) (uint64, error) {
	//First look in the cache
	id, ok := r.cache.Load(address)
	if ok {
		return id.(uint64), nil
	}

	a := models.Address{Address: address}
	_, err := r.db.Model(&a).
		Where("address = ?address").
		OnConflict("DO NOTHING").
		SelectOrInsert()
	if err != nil {
		return 0, err
	}
	r.cache.Store(address, a.ID)
	return a.ID, nil
}

func (r *Repository) FindOrCreateAll(addresses []string) ([]*models.Address, error) {
	var args []interface{}

	// Search in DB (use for update cache)
	result, _ := r.FindAll(addresses)

	for _, a := range addresses {
		_, exist := r.cache.Load(a)
		if !exist {
			args = append(args, &models.Address{Address: a})
		}
	}

	// if all addresses exists return it
	if len(args) == 0 {
		return result, nil
	}

	// create new addresses
	err := r.db.Insert(args...)
	if err != nil {
		return nil, err
	}

	return r.FindAll(addresses)
}

func (r *Repository) FindAll(addresses []string) ([]*models.Address, error) {
	var aList []*models.Address
	err := r.db.Model(&aList).Where(`address in (?)`, pg.In(addresses)).Select()
	r.addToCache(aList)
	return aList, err
}

func (r *Repository) addToCache(addresses []*models.Address) {
	for _, a := range addresses {
		r.cache.Store(a.Address, a.ID)
	}
}
