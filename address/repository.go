package address

import (
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/go-pg/pg/v10"
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
func (r *Repository) FindId(address string) (uint, error) {
	//First look in the cache
	id, ok := r.cache.Load(address)
	if ok {
		return id.(uint), nil
	}
	adr := new(models.Address)
	err := r.db.Model(adr).Column("id").Where("address = ?", address).Select(adr)
	if err != nil {
		return 0, err
	}
	r.cache.Store(address, adr.ID)
	return adr.ID, nil
}

//Find address id or create if not exist
func (r *Repository) FindIdOrCreate(address string) (uint, error) {
	//First look in the cache
	id, ok := r.cache.Load(address)
	if ok {
		return id.(uint), nil
	}

	adr := &models.Address{Address: address}
	_, err := r.db.Model(adr).
		Where("address = ?address").
		OnConflict("DO NOTHING").
		SelectOrInsert()

	if err != nil {
		return 0, err
	}

	r.cache.Store(address, adr.ID)
	return adr.ID, err
}

func (r *Repository) FindById(id uint) (string, error) {
	//First look in the cache
	address, ok := r.invCache.Load(id)
	if ok {
		return address.(string), nil
	}
	a := new(models.Address)
	err := r.db.Model(a).Where("id = ?", id).Select()
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

func (r *Repository) SaveAllIfNotExist(addresses []string) error {
	// if all addresses exists in cache do nothing
	loadFromDb := r.checkNotInCache(addresses)
	if len(loadFromDb) == 0 {
		return nil
	}
	var list []*models.Address      // need for cache update after insert
	_, err := r.FindAll(loadFromDb) //use for update cache
	if err != nil {
		return err
	}
	for _, a := range addresses {
		_, exist := r.cache.Load(a)
		if !exist {
			list = append(list, &models.Address{Address: a})
		}
	}
	// if all addresses do nothing
	if len(list) == 0 {
		return nil
	}
	_, err = r.db.Model(&list).Insert()
	if err != nil {
		r.addToCache(list)
	}
	return err
}

func (r *Repository) SaveFromMapIfNotExists(addresses map[string]struct{}) error {
	list := make([]string, len(addresses))
	i := 0
	for k := range addresses {
		list[i] = k
		i++
	}
	return r.SaveAllIfNotExist(list)
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
