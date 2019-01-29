package models

import (
	"math/big"
	"time"
)

type Block struct {
	ID                  uint64                `json:"id"`
	TotalTxs            uint64                `json:"total_txs"`
	Size                uint64                `json:"size"`
	ProposerValidatorID uint64                `json:"proposer_validator_id"`
	NumTxs              uint32                `json:"num_txs"`
	BlockTime           uint16                `json:"block_time"`
	CreatedAt           time.Time             `json:"created_at"`
	UpdatedAt           time.Time             `json:"updated_at"`
	BlockReward         big.Int               `json:"block_reward" sql:"type:numeric(70)"`
	Hash                string                `json:"hash"`
	Proposer            *Validator            `json:"proposer" pg:"fk:proposer_validator_id"`    //relation has one to Validators
	Validators          []*Validator          `json:"validators" pg:"many2many:block_validator"` //relation has many to Validators
	Transactions        []*Transaction        `json:"transactions"`                              //relation has many to Transactions
	InvalidTransactions []*InvalidTransaction `json:"invalid_transactions"`                      //relation has many to InvalidTransactions
	Rewards             []*Reward             `json:"rewards"`                                   //relation has many to Rewards
	Slashes             []*Slash              `json:"slashes"`                                   //relation has many to Slashes
}
