package models

type BlockValidator struct {
	tableName   struct{}  `sql:"block_validator"`
	BlockID     uint64    `json:"block_id"`
	ValidatorID uint64    `json:"validator_id"`
	Signed      *bool     `json:"signed"`
	Validator   Validator `json:"validator"`
}
