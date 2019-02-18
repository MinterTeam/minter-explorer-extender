package models

type Reward struct {
	BlockID     uint64 `json:"block_id"     sql:",pk"`
	AddressID   uint64 `json:"address_id"   sql:",pk"`
	ValidatorID uint64 `json:"validator_id" sql:",pk"`
	Role        string `json:"role"         sql:",pk"`
	Amount      string `json:"amount"       sql:"type:numeric(70)"`
}
