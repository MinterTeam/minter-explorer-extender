package liquidity_pool

import (
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/go-pg/pg/v10"
)

type Repository struct {
	db *pg.DB
}

func (r *Repository) getLiquidityPoolByCoinIds(firstCoinId, secondCoinId uint64) (*models.LiquidityPool, error) {
	var pl = new(models.LiquidityPool)
	err := r.db.Model(pl).Where("first_coin_id = ? AND second_coin_id = ?", firstCoinId, secondCoinId).Select()
	return pl, err
}

func (r *Repository) UpdateLiquidityPool(lp *models.LiquidityPool) error {
	_, err := r.db.Model(lp).OnConflict("(first_coin_id, second_coin_id) DO UPDATE").Insert()
	return err
}

func (r *Repository) LinkAddressLiquidityPool(addressId uint, liquidityPoolId uint64) error {
	addressLiquidityPool := &models.AddressLiquidityPool{
		LiquidityPoolId: liquidityPoolId,
		AddressId:       uint64(addressId),
	}
	_, err := r.db.Model(addressLiquidityPool).OnConflict("(address_id, liquidity_pool_id) DO NOTHING").Insert()
	return err
}

func (r *Repository) GetAddressLiquidityPool(addressId uint, liquidityPoolId uint64) (*models.AddressLiquidityPool, error) {
	var alp = new(models.AddressLiquidityPool)
	err := r.db.Model(alp).Where("address_id = ? AND liquidity_pool_id = ?", addressId, liquidityPoolId).Select()
	return alp, err
}

func (r *Repository) UpdateAddressLiquidityPool(alp *models.AddressLiquidityPool) error {
	_, err := r.db.Model(alp).OnConflict("(address_id, liquidity_pool_id) DO UPDATE").Insert()
	return err
}

func (r *Repository) DeleteAddressLiquidityPool(addressId uint, liquidityPoolId uint64) error {
	_, err := r.db.Model().Exec(`
		DELETE FROM address_liquidity_pools WHERE address_id = ? and liquidity_pool_id = ?;
	`, addressId, liquidityPoolId)
	return err
}

func NewRepository(db *pg.DB) *Repository {
	return &Repository{
		db: db,
	}
}
