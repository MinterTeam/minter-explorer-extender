package models

type Unbond struct {
	BlockId     uint       `json:"block_id"`
	AddressId   uint       `json:"address_id"`
	CoinId      uint       `json:"coin_id" pg:",use_zero"`
	ValidatorId uint       `json:"validator_id"`
	Value       string     `json:"value"`
	Coin        *Coin      `json:"coin"      pg:"fk:coin_id"`
	Address     *Address   `json:"address"   pg:"fk:address_id"`
	Validator   *Validator `json:"validator" pg:"fk:validator_id"`
}
