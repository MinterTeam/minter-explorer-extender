package models

import (
	"encoding/json"
	"fmt"
)

type Stake struct {
	ID             uint       `json:"id"               pg:",pk"`
	OwnerAddressID uint       `json:"owner_address_id"`
	ValidatorID    uint       `json:"validator_id"`
	CoinID         uint       `json:"coin_id"          pg:",use_zero"`
	Value          string     `json:"value"            pg:"type:numeric(70)"`
	BipValue       string     `json:"bip_value"        pg:"type:numeric(70)"`
	IsKicked       bool       `json:"is_kicked"`
	Coin           *Coin      `json:"coins"            pg:"rel:has-one"`                     //Relation has one to Coins
	OwnerAddress   *Address   `json:"owner_address"    pg:"rel:has-one,fk:owner_address_id"` //Relation has one to Addresses
	Validator      *Validator `json:"validator"        pg:"rel:has-one"`                     //Relation has one to Validators
}

func (s Stake) String() string {
	bytes, err := json.Marshal(&s)
	if err != nil {
		fmt.Println(err)
		return ""
	}
	return string(bytes)
}

type WLStake struct {
	tableName      struct{}   `pg:"wait_list"`
	OwnerAddressID uint       `json:"owner_address_id" pg:",pk"`
	ValidatorID    uint       `json:"validator_id"     pg:",pk"`
	CoinID         uint       `json:"coin_id"          pg:",use_zero,pk"`
	Value          string     `json:"value"            pg:"type:numeric(70)"`
	BipValue       string     `json:"bip_value"        pg:"type:numeric(70)"`
	Coin           *Coin      `json:"coins"            pg:"rel:has-one"`                     //Relation has one to Coins
	OwnerAddress   *Address   `json:"owner_address"    pg:"rel:has-one,fk:owner_address_id"` //Relation has one to Addresses
	Validator      *Validator `json:"validator"        pg:"rel:has-one"`                     //Relation has one to Validators
}
