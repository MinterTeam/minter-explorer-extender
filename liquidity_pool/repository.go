package liquidity_pool

import (
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/go-pg/pg/v10"
)

type Repository struct {
	db *pg.DB
}

func (r *Repository) getLiquidityPoolByCoinIds(firstCoin, secondCoin uint64) (*models.LiquidityPool, error) {
	var pl = new(models.LiquidityPool)
	err := r.db.Model(pl).Where("first_coin = ? AND second_coin = ?", firstCoin, secondCoin).Select()
	return pl, err
}

func (r *Repository) LiquidityPool(lp *models.LiquidityPool) error {
	_, err := r.db.Model(lp).OnConflict("(first_coin, second_coin) DO UPDATE").Insert()
	return err
}

func NewRepository(db *pg.DB) *Repository {
	return &Repository{
		db: db,
	}
}
