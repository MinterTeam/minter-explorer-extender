package models

type LiquidityPool struct {
	Id               uint64 `json:"id"`
	FirstCoinId      uint64 `json:"first_coin_id"  pg:",use_zero"`
	SecondCoinId     uint64 `json:"second_coin_id" pg:",use_zero"`
	FirstCoinVolume  string `json:"first_coin_volume"`
	SecondCoinVolume string `json:"second_coin_volume"`
	Liquidity        string `json:"liquidity"`
}

type AddressLiquidityPool struct {
	LiquidityPoolId uint64 `json:"liquidity_pool_id"`
	AddressId       uint64 `json:"address_id"`
	Liquidity       string `json:"liquidity"`
}
