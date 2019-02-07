package models

type Stake struct {
	ID             uint64     `json:"id"`
	OwnerAddressID uint64     `json:"owner_address_id"`
	ValidatorID    uint64     `json:"validator_id"`
	CoinID         uint64     `json:"coin_id"`
	Value          string     `json:"value"     sql:"type:numeric(70)"`
	BipValue       string     `json:"bip_value" sql:"type:numeric(70)"`
	Coin           *Coin      `json:"coins"`                                  //Relation has one to Coins
	OwnerAddress   *Address   `json:"owner_address" pg:"fk:owner_address_id"` //Relation has one to Addresses
	Validator      *Validator `json:"validator"`                              //Relation has one to Validators
}
