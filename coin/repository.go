package coin

import (
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/go-pg/pg/v9"
	"sync"
)

type Repository struct {
	db        *pg.DB
	cache     *sync.Map
	coinCache *sync.Map
}

func NewRepository(db *pg.DB) *Repository {
	return &Repository{
		db:        db,
		cache:     new(sync.Map), //TODO: добавить реализацию очистки
		coinCache: new(sync.Map), //TODO: добавить реализацию очистки
	}
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

func (r *Repository) FindSymbolById(id uint) (string, error) {
	//First look in the cache
	data, ok := r.coinCache.Load(id)
	if ok {
		return data.(*models.Coin).Symbol, nil
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
	r.coinCache.Store(id, coin)
	return coin.Symbol, nil
}

func (r *Repository) GetById(id uint) (*models.Coin, error) {
	//First look in the cache
	data, ok := r.coinCache.Load(id)
	if ok {
		return data.(*models.Coin), nil
	}

	coin := &models.Coin{ID: id}
	err := r.db.Model(coin).
		Where("id = ?", id).
		Limit(1).
		Select()

	if err != nil {
		return nil, err
	}

	r.cache.Store(coin.Symbol, id)
	r.coinCache.Store(id, coin)
	return coin, nil
}

func (r *Repository) Save(c *models.Coin) error {
	_, err := r.db.Model(c).
		Where("symbol = ?symbol").
		OnConflict("(symbol, version) DO UPDATE").
		SelectOrInsert()
	if err != nil {
		return err
	}
	r.cache.Store(c.Symbol, c.ID)
	return nil
}

func (r *Repository) Update(c *models.Coin) error {
	_, err := r.db.Model(c).WherePK().Update()
	return err
}

func (r *Repository) Add(c *models.Coin) error {
	_, err := r.db.Model(c).Insert()
	return err
}

func (r Repository) SaveAllIfNotExist(coins []*models.Coin) error {
	_, err := r.db.Model(&coins).OnConflict("(symbol, version) DO UPDATE").Insert()
	if err != nil {
		return err
	}
	for _, coin := range coins {
		r.cache.Store(coin.Symbol, coin.ID)
		r.coinCache.Store(coin.ID, coin)
	}
	return err
}

func (r Repository) SaveAllNewIfNotExist(coins []*models.Coin) error {
	_, err := r.db.Model(&coins).OnConflict("(symbol, version) DO UPDATE").Insert()
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

func (r *Repository) GetCoinBySymbol(symbol string) ([]models.Coin, error) {
	var coins []models.Coin
	err := r.db.Model(&coins).Where("symbol = ?", symbol).Select()
	return coins, err
}

func (r *Repository) RemoveFromCacheBySymbol(symbol string) {
	id, ok := r.cache.Load(symbol)
	if ok {
		r.cache.Delete(symbol)
		r.coinCache.Delete(id)
	}
}

func (r *Repository) GetLastCoinId() (uint, error) {
	coin := new(models.Coin)

	err := r.db.Model(coin).
		Order("id desc").
		Limit(1).
		Select()

	return coin.ID, err
}

func (r *Repository) UpdateAll(coins []*models.Coin) error {

	for _, c := range coins {
		_, err := r.db.Model(c).WherePK().Update()
		if err != nil {
			return err
		}
	}
	return nil
}
