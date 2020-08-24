package models

type StakeKick struct {
	AddressId   uint     `json:"address_id"`
	CoinId      uint     `json:"coin_id"`
	ValidatorId uint     `json:"validator_id"`
	Amount      string   `json:"amount"`
	tableName   struct{} `pg:"wait_list"`
}
