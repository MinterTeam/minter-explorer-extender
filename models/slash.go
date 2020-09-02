package models

const SlashEvent = "minter/SlashEvent"

type Slash struct {
	ID          uint64     `json:"id"            pg:",pk"`
	CoinID      uint       `json:"coin_id"       pg:",use_zero"`
	BlockID     uint64     `json:"block_id"`
	AddressID   uint       `json:"address_id"`
	ValidatorID uint64     `json:"validator_id"`
	Amount      string     `json:"amount"        pg:"type:numeric(70)"`
	Coin        *Coin      `pg:"fk:coin_id"` //Relation has one to Coins
	Block       *Block     //Relation has one to Blocks
	Address     *Address   //Relation has one to Addresses
	Validator   *Validator //Relation has one to Validators
}
