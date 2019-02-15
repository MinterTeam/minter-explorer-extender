package coin

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

//Find address or create if not exist
func (r *Repository) FindIdBySymbol(symbol string) (uint64, error) {
	//First look in the cache
	id, ok := r.cache.Load(symbol)
	if ok {
		return id.(uint64), nil
	}
	coin := new(models.Coin)
	err := r.db.Model(coin).
		Where("symbol = ?", symbol).
		Where("deleted_at_block_id isnull").
		Select()

	if err != nil {
		return 0, err
	}
	r.cache.Store(symbol, coin.ID)
	return coin.ID, nil
}

func (r Repository) Save(c *models.Coin) error {
	err := r.db.Insert(c)
	if err != nil {
		return err
	}
	r.cache.Store(c.Symbol, c.ID)
	return nil
}
