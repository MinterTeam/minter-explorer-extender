package models

type OrderType byte

const (
	_ OrderType = iota
	OrderTypeNew
	OrderTypeActive
	OrderTypePartiallyFilled
	OrderTypeFilled
	OrderTypeCanceled
	OrderTypeExpired
)

type Order struct {
	Id              uint64         `json:"id"`
	AddressId       uint64         `json:"address_id"`
	LiquidityPoolId uint64         `json:"liquidity_pool_id"`
	Price           string         `json:"price"             pg:"type:numeric(100)"`
	CoinSellId      uint64         `json:"coin_sell_id"      pg:",use_zero"`
	CoinSellVolume  string         `json:"coin_sell_volume"  pg:"type:numeric(100)"`
	CoinBuyId       uint64         `json:"coin_buy_id"       pg:",use_zero"`
	CoinBuyVolume   string         `json:"coin_buy_volume"   pg:"type:numeric(100)"`
	CreatedAtBlock  uint64         `json:"created_at_block"`
	Status          OrderType      `json:"status"`
	Address         *Address       `json:"address"           pg:"rel:has-one,fk:address_id"`
	LiquidityPool   *LiquidityPool `json:"liquidity_pool"    pg:"rel:has-one,fk:liquidity_pool_id"`
	CoinSell        *Coin          `json:"coin_sell"         pg:"rel:has-one,fk:coin_sell_id"`
	CoinBuy         *Coin          `json:"coin_buy"          pg:"rel:has-one,fk:coin_buy_id"`
}
