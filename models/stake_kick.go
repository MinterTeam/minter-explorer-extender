package models

type StakeKick struct {
	tableName   struct{}   `pg:"wait_list"`
	AddressId   uint       `json:"address_id"`
	CoinId      uint       `json:"coin_id"`
	ValidatorId uint       `json:"validator_id"`
	Value       string     `json:"value"`
	Coin        *Coin      `json:"coin"      pg:"fk:coin_id"`
	Validator   *Validator `json:"validator" pg:"fk:validator_id"`
}
