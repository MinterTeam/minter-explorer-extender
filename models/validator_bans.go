package models

type ValidatorBan struct {
	ValidatorId uint   `json:"validator_id"  pg:"rel:belongs-to"`
	BlockId     uint64 `json:"block_id"`
}
