package models

type ValidatorBan struct {
	ValidatorId uint       `json:"validator_id"`
	BlockId     uint64     `json:"block_id"`
	Validator   *Validator `json:"validator"  pg:"rel:belongs-to"`
}
