package coin

import (
	"fmt"
	"github.com/MinterTeam/minter-explorer-tools/v4/models"
	"github.com/go-pg/pg/v9"
	"os"
	"sync"
)

type Repository struct {
	db       *pg.DB
	cache    *sync.Map
	invCache *sync.Map
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
		db:       db,
		cache:    new(sync.Map), //TODO: добавить реализацию очистки
		invCache: new(sync.Map), //TODO: добавить реализацию очистки
	}
}

// Find coin id by symbol
func (r *Repository) FindIdBySymbol(symbol string) (uint64, error) {
	//First look in the cache
	id, ok := r.cache.Load(symbol)
	if ok {
		return id.(uint64), nil
	}
	coin := new(models.Coin)
	err := r.db.Model(coin).
		Column("id").
		Where("symbol = ?", symbol).
		AllWithDeleted().
		Select()

	if err != nil {
		return 0, err
	}
	r.cache.Store(symbol, coin.ID)
	return coin.ID, nil
}

func (r *Repository) FindSymbolById(id uint64) (string, error) {
	//First look in the cache
	symbol, ok := r.invCache.Load(id)
	if ok {
		return symbol.(string), nil
	}
	coin := &models.Coin{ID: id}
	err := r.db.Model(coin).
		Where("id = ?", id).
		Limit(1).
		Select()

	if err != nil {
		return "", err
	}
	r.cache.Store(coin.Symbol, id)
	r.invCache.Store(id, coin.Symbol)
	return coin.Symbol, nil
}

func (r *Repository) Save(c *models.Coin) error {
	_, err := r.db.Model(c).
		Where("symbol = ?symbol").
		OnConflict("DO NOTHING"). //TODO: change to DO UPDATE
		SelectOrInsert()
	if err != nil {
		return err
	}
	r.cache.Store(c.Symbol, c.ID)
	return nil
}

func (r Repository) SaveAllIfNotExist(coins []*models.Coin) error {
	_, err := r.db.Model(&coins).OnConflict("(symbol) DO UPDATE").Insert()
	if err != nil {
		return err
	}
	for _, coin := range coins {
		r.cache.Store(coin.Symbol, coin.ID)
		r.invCache.Store(coin.ID, coin.Symbol)
	}
	return err
}

func (r *Repository) GetAllCoins() ([]*models.Coin, error) {
	var coins []*models.Coin
	err := r.db.Model(&coins).Order("symbol ASC").Select()
	return coins, err
}

func (r Repository) DeleteBySymbol(symbol string) error {
	coin := &models.Coin{Symbol: symbol}
	_, err := r.db.Model(coin).Where("symbol = ?symbol").Delete()
	return err
}
