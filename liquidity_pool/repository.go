package liquidity_pool

import (
	"encoding/json"
	"fmt"
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/go-pg/pg/v10"
	"strconv"
	"strings"
	"time"
)

type Repository struct {
	db *pg.DB
}

func (r *Repository) GetPoolsByTxTags(tags map[string]string) ([]models.LiquidityPool, error) {
	pools, err := r.getPoolChainFromTags(tags)
	if err != nil {
		return nil, err
	}
	var idList []uint64
	for id := range pools {
		idList = append(idList, id)
	}
	return r.GetAllByIds(idList)
}

func (r *Repository) GetPoolByPairString(pair string) (*models.LiquidityPool, error) {
	ids := strings.Split(pair, "-")
	firstCoinId, err := strconv.ParseUint(ids[0], 10, 64)
	if err != nil {
		return nil, err
	}
	secondCoinId, err := strconv.ParseUint(ids[1], 10, 64)
	if err != nil {
		return nil, err
	}
	if firstCoinId < secondCoinId {
		return r.getLiquidityPoolByCoinIds(firstCoinId, secondCoinId)
	} else {
		return r.getLiquidityPoolByCoinIds(secondCoinId, firstCoinId)
	}
}

func (r *Repository) getLiquidityPoolByCoinIds(firstCoinId, secondCoinId uint64) (*models.LiquidityPool, error) {
	var lp = new(models.LiquidityPool)
	err := r.db.Model(lp).Where("first_coin_id = ? AND second_coin_id = ?", firstCoinId, secondCoinId).Select()
	return lp, err
}

func (r *Repository) getLiquidityPoolByTokenId(id uint64) (*models.LiquidityPool, error) {
	var lp = new(models.LiquidityPool)
	err := r.db.Model(lp).Where("token_id = ?", id).Select()
	return lp, err
}

func (r *Repository) getLiquidityPoolById(id uint64) (*models.LiquidityPool, error) {
	var lp = new(models.LiquidityPool)
	err := r.db.Model(lp).Where("id = ?", id).Select()
	return lp, err
}

func (r *Repository) UpdateLiquidityPool(lp *models.LiquidityPool) error {
	_, err := r.db.Model(lp).OnConflict("(first_coin_id, second_coin_id) DO UPDATE").Insert()
	return err
}

func (r *Repository) UpdateLiquidityPoolById(lp *models.LiquidityPool) error {
	_, err := r.db.Model(lp).
		Column("liquidity").
		Column("first_coin_volume").
		Column("second_coin_volume").
		Column("updated_at_block_id").
		WherePK().
		Update()
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

func (r *Repository) GetAddressLiquidityPoolByCoinId(addressId uint, liquidityPoolId uint64) (*models.AddressLiquidityPool, error) {
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

func (r *Repository) UpdateAllLiquidityPool(pools []*models.AddressLiquidityPool) error {
	_, err := r.db.Model(&pools).OnConflict("(address_id, liquidity_pool_id) DO UPDATE").Insert()
	return err
}

func (r *Repository) GetAllByIds(ids []uint64) ([]models.LiquidityPool, error) {
	var list []models.LiquidityPool
	err := r.db.Model(&list).Where("id in (?)", pg.In(ids)).Select()
	return list, err
}

func (r *Repository) SaveAllLiquidityPoolTrades(links []*models.LiquidityPoolTrade) error {
	_, err := r.db.Model(&links).Insert()
	return err
}

func (r *Repository) GetAll() ([]models.LiquidityPool, error) {
	var list []models.LiquidityPool
	err := r.db.Model(&list).Select()
	return list, err
}

func (r *Repository) GetLastSnapshot() (*models.LiquidityPoolSnapshot, error) {
	var lps = new(models.LiquidityPoolSnapshot)
	err := r.db.Model(lps).Order("block_id desc").Limit(1).Select()
	return lps, err
}

func (r *Repository) GetSnapshotsByDate(date time.Time) ([]models.LiquidityPoolSnapshot, error) {
	var list []models.LiquidityPoolSnapshot
	startDate := fmt.Sprintf("%s 00:00:00", date.Format("2006-01-02"))
	endDate := fmt.Sprintf("%s 23:59:59", date.Format("2006-01-02"))
	err := r.db.Model(&list).Where("created_at >= ? and created_at <= ?", startDate, endDate).Select()
	return list, err
}

func (r *Repository) SaveLiquidityPoolSnapshots(snap []models.LiquidityPoolSnapshot) error {
	_, err := r.db.Model(&snap).Insert()
	return err
}

func (r *Repository) RemoveEmptyAddresses() error {
	_, err := r.db.Model().Exec(`DELETE FROM address_liquidity_pools WHERE liquidity <= 0;`)
	return err
}

func (r *Repository) getPoolChainFromTags(tags map[string]string) (map[uint64][]map[string]string, error) {
	var poolsData []models.TagLiquidityPool
	err := json.Unmarshal([]byte(tags["tx.pools"]), &poolsData)
	if err != nil {
		return nil, err
	}

	data := make(map[uint64][]map[string]string)
	for _, p := range poolsData {
		firstCoinData := make(map[string]string)
		firstCoinData["coinId"] = fmt.Sprintf("%d", p.CoinIn)
		firstCoinData["volume"] = p.ValueIn

		secondCoinData := make(map[string]string)
		secondCoinData["coinId"] = fmt.Sprintf("%d", p.CoinOut)
		secondCoinData["volume"] = p.ValueIn

		data[p.PoolID] = []map[string]string{firstCoinData, secondCoinData}
	}
	return data, nil
}

func NewRepository(db *pg.DB) *Repository {
	return &Repository{
		db: db,
	}
}
