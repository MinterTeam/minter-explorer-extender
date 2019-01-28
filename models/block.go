package models

import "time"

type Block struct {
	ID                  uint64       `json:"id"`
	TotalTxs            uint64       `json:"total_txs"`
	Size                uint64       `json:"size"`
	ProposerValidatorID uint64       `json:"proposer_validator_id"`
	NumTxs              uint32       `json:"num_txs"`
	BlockTime           uint16       `json:"block_time"`
	CreatedAt           time.Time    `json:"created_at"`
	UpdatedAt           time.Time    `json:"updated_at"`
	BlockReward         string       `json:"block_reward"`
	Hash                string       `json:"hash"`
	Validators          []*Validator `json:"validators" pg:"many2many:block_validator,joinFK:validator_id"`
}
