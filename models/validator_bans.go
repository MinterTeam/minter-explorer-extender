package models

type ValidatorBan struct {
	ValidatorId uint   `json:"validator_id"`
	BlockId     uint64 `json:"block_id"`
}
