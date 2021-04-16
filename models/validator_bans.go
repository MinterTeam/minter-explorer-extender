package models

type ValidatorBan struct {
	ValidatorId uint       `json:"validator_id" pg:",pk"`
	BlockId     uint64     `json:"block_id"     pg:",pk"`
	Validator   *Validator `json:"validator"    pg:"rel:belongs-to"`
	Block       *Block     `json:"block"        pg:"rel:belongs-to"`
}
