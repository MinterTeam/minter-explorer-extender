package models

type Order struct {
	Id              uint64 `json:"id"`
	AddressId       uint64 `json:"address_id"`
	LiquidityPoolId uint64 `json:"liquidity_pool_id"`
	CoinSellId      uint64 `json:"coin_sell_id"`
	CoinSellVolume  string `json:"coin_sell_volume"`
	CoinBuyId       uint64 `json:"coin_buy_id"`
	CoinBuyVolume   string `json:"coin_buy_volume"`
}
