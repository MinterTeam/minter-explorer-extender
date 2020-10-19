package models

const RewardEvent = "minter/RewardEvent"

type Reward struct {
	BlockID     uint64     `json:"block"        pg:",pk"`
	AddressID   uint       `json:"address_id"   pg:",pk"`
	ValidatorID uint64     `json:"validator_id" pg:",pk"`
	Role        string     `json:"role"         pg:",pk"`
	Amount      string     `json:"amount"       pg:"type:numeric(70)"`
	Block       *Block     //Relation has one to Blocks
	Address     *Address   //Relation has one to Addresses
	Validator   *Validator //Relation has one to Validators
}
