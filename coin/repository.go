package coin

import (
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/go-pg/pg/v9"
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

// Find coin id by symbol
func (r *Repository) FindIdBySymbol(symbol string) (uint, error) {
	coin := new(models.Coin)
	err := r.db.Model(coin).
		Where("symbol = ? and version = ?", symbol, 0).
		AllWithDeleted().
		Select()

	if err != nil {
		return 0, err
	}
	return coin.ID, nil
}

// Find coin id by symbol
func (r *Repository) FindCoinIdBySymbol(symbol string) (uint, error) {
	//First look in the cache
	id, ok := r.cache.Load(symbol)
	if ok {
		return id.(uint), nil
	}
	coin := new(models.Coin)
	err := r.db.Model(coin).
		Column("coin_id").
		Where("symbol = ?", symbol).
		AllWithDeleted().
		Select()

	if err != nil {
		return 0, err
	}
	r.cache.Store(symbol, coin.CoinId)
	return coin.CoinId, nil
}

func (r *Repository) FindSymbolById(id uint) (string, error) {
	//First look in the cache
	symbol, ok := r.invCache.Load(id)
	if ok {
		return symbol.(string), nil
	}
	coin := &models.Coin{CoinId: id}
	err := r.db.Model(coin).
		Where("coin_id = ?", id).
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
	r.cache.Store(c.Symbol, c.CoinId)
	return nil
}

func (r *Repository) Update(c *models.Coin) error {
	_, err := r.db.Model(c).WherePK().Update()
	return err
}

func (r *Repository) Add(c *models.NewCoin) error {
	_, err := r.db.Model(c).Insert()
	return err
}

func (r Repository) SaveAllIfNotExist(coins []*models.Coin) error {
	_, err := r.db.Model(&coins).OnConflict("DO NOTHING").Insert()
	if err != nil {
		return err
	}
	for _, coin := range coins {
		r.cache.Store(coin.Symbol, coin.CoinId)
		r.invCache.Store(coin.CoinId, coin.Symbol)
	}
	return err
}

func (r Repository) SaveAllNewIfNotExist(coins []*models.NewCoin) error {
	_, err := r.db.Model(&coins).OnConflict("DO NOTHING").Insert()
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

func (r *Repository) UpdateOwnerBySymbol(symbol string, id uint) error {
	_, err := r.db.Model().Exec(`
		UPDATE coins SET owner_address_id = ?
		WHERE symbol = ?;
	`, id, symbol)
	return err
}

func (r *Repository) GetNewCoins() ([]models.Coin, error) {
	var coins []models.Coin
	err := r.db.Model(&coins).Where("coin_id is null").Select()
	return coins, err
}

func (r *Repository) GetCoinBySymbol(symbol string) ([]models.Coin, error) {
	var coins []models.Coin
	err := r.db.Model(&coins).Where("symbol = ?", symbol).Select()
	return coins, err
}

func (r *Repository) RemoveFromCacheBySymbol(symbol string) {
	id, ok := r.cache.Load(symbol)
	if ok {
		r.cache.Delete(symbol)
		r.invCache.Delete(id)
	}
}
