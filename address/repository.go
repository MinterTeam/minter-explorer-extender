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
		cache: new(sync.Map),
	}
}

//Find address id or create if not exist
func (r *Repository) FindIdOrCreate(address string) (uint64, error) {
	//First look in the cache
	id, ok := r.cache.Load(address)
	if ok {
		return id.(uint64), nil
	} else {
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
}
