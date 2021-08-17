package models

import "time"

type LiquidityPoolTrade struct {
	BlockId          uint64         `json:"block_id"`
	LiquidityPoolId  uint64         `json:"liquidity_pool_id"`
	TransactionId    uint64         `json:"transaction_id"`
	FirstCoinVolume  string         `json:"first_coin_volume"`
	SecondCoinVolume string         `json:"second_coin_volume"`
	CreatedAt        time.Time      `json:"created_at"`
	Block            *Block         `json:"block"          pg:"rel:has-one,fk:block_id"`
	LiquidityPool    *LiquidityPool `json:"liquidity_pool" pg:"rel:has-one,fk:liquidity_pool_id"`
	Transaction      *Transaction   `json:"transaction"    pg:"rel:has-one,fk:transaction_id"`
}
