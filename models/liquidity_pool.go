package models

import (
	"fmt"
	"time"
)

const LockedLiquidityVolume = 1000

type LiquidityPool struct {
	Id               uint64 `json:"id"                 pg:",pk"`
	TokenId          uint64 `json:"token_id"`
	FirstCoinId      uint64 `json:"first_coin_id"      pg:",use_zero"`
	SecondCoinId     uint64 `json:"second_coin_id"     pg:",use_zero"`
	FirstCoinVolume  string `json:"first_coin_volume"  pg:"type:numeric(100)"`
	SecondCoinVolume string `json:"second_coin_volume" pg:"type:numeric(100)"`
	Liquidity        string `json:"liquidity"`
	LiquidityBip     string `json:"liquidity_bip"`
	UpdatedAtBlockId uint64 `json:"updated_at_block_id"`
	FirstCoin        *Coin  `json:"first_coin"  pg:"rel:has-one,fk:first_coin_id"`
	SecondCoin       *Coin  `json:"second_coin" pg:"rel:has-one,fk:second_coin_id"`
	Token            *Coin  `json:"token"       pg:"rel:has-one,fk:token_id"`
}

func (lp *LiquidityPool) GetTokenSymbol() string {
	return fmt.Sprintf("LP-%d", lp.Id)
}

type AddressLiquidityPool struct {
	LiquidityPoolId  uint64         `json:"liquidity_pool_id" pg:",pk"`
	AddressId        uint64         `json:"address_id"        pg:",pk"`
	FirstCoinVolume  string         `json:"first_coin_volume"  pg:"type:numeric(100)"`
	SecondCoinVolume string         `json:"second_coin_volume" pg:"type:numeric(100)"`
	Liquidity        string         `json:"liquidity"`
	Address          *Address       `json:"address"           pg:"rel:has-one,fk:address_id"`
	LiquidityPool    *LiquidityPool `json:"liquidity_pool"    pg:"rel:has-one,fk:liquidity_pool_id"`
}

type TagLiquidityPool struct {
	PoolID   uint64 `json:"pool_id"`
	CoinIn   uint64 `json:"coin_in"`
	ValueIn  string `json:"value_in"`
	CoinOut  uint64 `json:"coin_out"`
	ValueOut string `json:"value_out"`
}

type LiquidityPoolSnapshot struct {
	BlockId          uint64    `json:"block_id"`
	LiquidityPoolId  uint64    `json:"liquidity_pool_id"`
	FirstCoinVolume  string    `json:"first_coin_volume"`
	SecondCoinVolume string    `json:"second_coin_volume"`
	Liquidity        string    `json:"liquidity"`
	LiquidityBip     string    `json:"liquidity_bip"`
	CreatedAt        time.Time `json:"created_at"`
}
