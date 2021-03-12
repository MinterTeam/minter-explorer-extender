package models

type Unbond struct {
	BlockId     uint       `json:"block_id"`
	AddressId   uint       `json:"address_id"`
	CoinId      uint       `json:"coin_id" pg:",use_zero"`
	ValidatorId uint       `json:"validator_id"`
	Value       string     `json:"value"`
	Coin        *Coin      `json:"coin"      pg:"rel:has-one,fk:coin_id"`
	Address     *Address   `json:"address"   pg:"rel:has-one,fk:address_id"`
	Validator   *Validator `json:"validator" pg:"rel:has-one,fk:validator_id"`
}
