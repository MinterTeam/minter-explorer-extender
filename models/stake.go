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

type MovedStake struct {
	BlockId         uint64     `json:"block_id"`
	AddressId       uint64     `json:"address_id"`
	CoinId          uint64     `json:"coin_id" pg:",use_zero"`
	FromValidatorId uint64     `json:"from_validator_id"`
	ToValidatorId   uint64     `json:"to_validator_id"`
	Value           string     `json:"value"`
	Coin            *Coin      `json:"coin"           pg:"rel:has-one,fk:coin_id"`
	Address         *Address   `json:"address"        pg:"rel:has-one,fk:address_id"`
	FromValidator   *Validator `json:"from_validator" pg:"rel:has-one,fk:from_validator_id"`
	ToValidator     *Validator `json:"to_validator"   pg:"rel:has-one,fk:to_validator_id"`
}
