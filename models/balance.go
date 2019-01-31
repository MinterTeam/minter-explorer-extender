package models

import "math/big"

type Balance struct {
	ID        uint64   `json:"id"`
	AddressID uint64   `json:"address_id"`
	CoinID    uint64   `json:"coin_id"`
	Value     big.Int  `json:"value" sql:"type:numeric(70)"`
	Address   *Address //Relation has one to Address
	Coin      *Coin    //Relation has one to Coin
}
