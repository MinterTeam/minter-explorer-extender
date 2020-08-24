package models

type Balance struct {
	ID        uint     `json:"id"          pg:",pk"`
	AddressID uint     `json:"address_id"`
	CoinID    uint     `json:"coin_id"     pg:",use_zero"`
	Value     string   `json:"value"       pg:"type:numeric(70)"`
	Address   *Address //Relation has one to Address
	Coin      *Coin    `pg:"fk:id"` //Relation has one to Coin
}
