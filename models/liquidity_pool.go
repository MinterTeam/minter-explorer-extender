package models

import (
	"fmt"
)

const LockedLiquidityVolume = 1000

type LiquidityPool struct {
	Id               uint64 `json:"id"`
	TokenId          uint64 `json:"token_id"`
	FirstCoinId      uint64 `json:"first_coin_id"  pg:",use_zero"`
	SecondCoinId     uint64 `json:"second_coin_id" pg:",use_zero"`
	FirstCoinVolume  string `json:"first_coin_volume"`
	SecondCoinVolume string `json:"second_coin_volume"`
	Liquidity        string `json:"liquidity"`
	LiquidityBip     string `json:"liquidity_bip"`
	FirstCoin        *Coin  `json:"first_coin"  pg:"rel:has-one,fk:first_coin_id"`
	SecondCoin       *Coin  `json:"second_coin" pg:"rel:has-one,fk:second_coin_id"`
	Token            *Coin  `json:"token"       pg:"rel:has-one,fk:token_id"`
}

type AddressLiquidityPool struct {
	LiquidityPoolId uint64         `json:"liquidity_pool_id" pg:",pk"`
	AddressId       uint64         `json:"address_id"        pg:",pk"`
	Liquidity       string         `json:"liquidity"`
	Address         *Address       `json:"first_coin"        pg:"rel:has-one,fk:address_id"`
	LiquidityPool   *LiquidityPool `json:"liquidity_pool"    pg:"rel:has-one,fk:liquidity_pool_id"`
}

type TagLiquidityPool struct {
	PoolID   uint64 `json:"pool_id"`
	CoinIn   uint64 `json:"coin_in"`
	ValueIn  string `json:"value_in"`
	CoinOut  uint64 `json:"coin_out"`
	ValueOut string `json:"value_out"`
}

func (lp *LiquidityPool) GetTokenSymbol() string {
	return fmt.Sprintf("LP-%d", lp.Id)
}
