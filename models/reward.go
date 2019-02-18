package models

const RewardEvent = "minter/RewardEvent"

type Reward struct {
	BlockID     uint64     `json:"block"        sql:",pk"`
	AddressID   uint64     `json:"address_id"   sql:",pk"`
	ValidatorID uint64     `json:"validator_id" sql:",pk"`
	Role        string     `json:"role"         sql:",pk"`
	Amount      string     `json:"amount"       sql:"type:numeric(70)"`
	Block       *Block     //Relation has one to Blocks
	Address     *Address   //Relation has one to Addresses
	Validator   *Validator //Relation has one to Validators
}
