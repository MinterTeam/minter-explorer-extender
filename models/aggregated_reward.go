package models

import "time"

type AggregatedReward struct {
	FromBlockID uint64     `json:"from_block_id" pg:",pk"`
	ToBlockID   uint64     `json:"to_block_id"`
	AddressID   uint64     `json:"address_id"    pg:",pk"`
	ValidatorID uint64     `json:"validator_id"  pg:",pk"`
	Role        string     `json:"role"          pg:",pk"`
	Amount      string     `json:"amount"        pg:"type:numeric(70)"`
	TimeID      time.Time  `json:"time_id"`
	FromBlock   *Block     `pg:"rel:has-one,fk:from_block_id"` //Relation has one to Blocks
	ToBlock     *Block     `pg:"rel:has-one,fk:to_block_id"`   //Relation has one to Blocks
	Address     *Address   `pg:"rel:has-one"`                  //Relation has one to Addresses
	Validator   *Validator `pg:"rel:has-one"`                  //Relation has one to Validators
}
