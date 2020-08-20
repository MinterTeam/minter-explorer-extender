package models

type Stake struct {
	ID             uint       `json:"id"               pg:",pk"`
	OwnerAddressID uint       `json:"owner_address_id"`
	ValidatorID    uint       `json:"validator_id"`
	CoinID         uint       `json:"coin_id"          pg:",use_zero"`
	Value          string     `json:"value"            pg:"type:numeric(70)"`
	BipValue       string     `json:"bip_value"        pg:"type:numeric(70)"`
	Coin           *Coin      `json:"coins"`                                  //Relation has one to Coins
	OwnerAddress   *Address   `json:"owner_address" pg:"fk:owner_address_id"` //Relation has one to Addresses
	Validator      *Validator `json:"validator"`                              //Relation has one to Validators
}
