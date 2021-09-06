package models

type ValidatorBan struct {
	ValidatorId uint       `json:"validator_id" pg:",pk"`
	BlockId     uint64     `json:"block_id"     pg:",pk"`
	ToBlockId   uint64     `json:"to_block_id"`
	Validator   *Validator `json:"validator"    pg:"rel:has-one"`
	Block       *Block     `json:"block"        pg:"rel:has-one"`
	ToBlock     *Block     `json:"to_block"     pg:"rel:has-one,fk:to_block_id"`
}
