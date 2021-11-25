package models

type BuySwapPoolTag struct {
	PoolId   uint64       `json:"pool_id"`
	CoinIn   uint64       `json:"coin_in"`
	ValueIn  string       `json:"value_in"`
	CoinOut  uint64       `json:"coin_out"`
	ValueOut string       `json:"value_out"`
	Details  TxTagDetails `json:"details"`
	Sellers  []struct {
		Seller string `json:"seller"`
		Value  string `json:"value"`
	} `json:"sellers"`
}

type TxTagDetails struct {
	AmountIn            string              `json:"amount_in"`
	AmountOut           string              `json:"amount_out"`
	CommissionAmountIn  string              `json:"commission_amount_in"`
	CommissionAmountOut string              `json:"commission_amount_out"`
	Orders              []TxTagDetailsOrder `json:"orders"`
}

type TxTagDetailsOrder struct {
	Buy    string `json:"buy"`
	Sell   string `json:"sell"`
	Seller string `json:"seller"`
	Id     uint64 `json:"id"`
}

type TagCommissionDetails struct {
	PoolID   uint64                        `json:"pool_id"`
	CoinIn   uint64                        `json:"coin_in"`
	ValueIn  string                        `json:"value_in"`
	CoinOut  uint64                        `json:"coin_out"`
	ValueOut string                        `json:"value_out"`
	Details  CommissionDetails             `json:"details"`
	Sellers  []CommissionDetailOrderSeller `json:"sellers"`
}

type CommissionDetails struct {
	AmountIn            string              `json:"amount_in"`
	AmountOut           string              `json:"amount_out"`
	CommissionAmountIn  string              `json:"commission_amount_in"`
	CommissionAmountOut string              `json:"commission_amount_out"`
	Orders              []TxTagDetailsOrder `json:"orders"`
}

type CommissionDetailOrderSeller struct {
	Seller string `json:"seller"`
	Value  string `json:"value"`
}
