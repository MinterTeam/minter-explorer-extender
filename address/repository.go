package address

import (
	"github.com/MinterTeam/minter-explorer-extender/models"
	"github.com/go-pg/pg"
	"sync"
)

type Repository struct {
	db       *pg.DB
	cache    *sync.Map
	invCache *sync.Map
}

func NewRepository(db *pg.DB) *Repository {
	return &Repository{
		db:       db,
		cache:    new(sync.Map), //TODO: добавить реализацию очистки
		invCache: new(sync.Map), //TODO: добавить реализацию очистки
	}
}

//Find address id
func (r *Repository) FindId(address string) (uint64, error) {
	//First look in the cache
	id, ok := r.cache.Load(address)
	if ok {
		return id.(uint64), nil
	}
	a := new(models.Address)
	err := r.db.Model(a).Where("address = ?", address).Select(a)
	if err != nil {
		return 0, err
	}
	r.cache.Store(address, a.ID)
	return a.ID, nil
}

func (r *Repository) FindById(id uint64) (string, error) {
	//First look in the cache
	address, ok := r.invCache.Load(id)
	if ok {
		return address.(string), nil
	}
	a := &models.Address{ID: id}
	err := r.db.Select(a)
	if err != nil {
		return "", err
	}
	r.cache.Store(a.Address, id)
	r.invCache.Store(id, a.Address)
	return a.Address, nil
}

func (r *Repository) FindAll(addresses []string) ([]*models.Address, error) {
	var aList []*models.Address
	err := r.db.Model(&aList).Where(`address in (?)`, pg.In(addresses)).Select()
	if err != nil {
		return nil, err
	}
	r.addToCache(aList)
	return aList, err
}

func (r Repository) SaveAllIfNotExist(addresses []string) error {
	// if all addresses exists in cache do nothing
	loadFromDb := r.checkNotInCache(addresses)
	if len(loadFromDb) == 0 {
		return nil
	}
	var args []interface{}
	var aList []*models.Address  // need for cache update after insert
	_, _ = r.FindAll(loadFromDb) //use for update cache
	for _, a := range addresses {
		_, exist := r.cache.Load(a)
		if !exist {
			address := &models.Address{Address: a}
			args = append(args, address)
			aList = append(aList, address)
		}
	}
	// if all addresses do nothing
	if len(args) == 0 {
		return nil
	}
	err := r.db.Insert(args...)
	if err != nil {
		r.addToCache(aList)
	}
	return err
}

func (r *Repository) addToCache(addresses []*models.Address) {
	for _, a := range addresses {
		_, exist := r.cache.Load(a)
		if !exist {
			r.cache.Store(a.Address, a.ID)
			r.invCache.Store(a.ID, a.Address)
		}
	}
}

func (r *Repository) checkNotInCache(addresses []string) []string {
	var list []string
	for _, a := range addresses {
		_, exist := r.cache.Load(a)
		if !exist {
			list = append(list, a)
		}
	}
	return list
}
