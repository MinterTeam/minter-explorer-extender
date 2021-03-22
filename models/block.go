package models

import "time"

type Block struct {
	ID                  uint64                `json:"id"                   pg:",pk"`
	Size                uint64                `json:"size"`
	ProposerValidatorID uint64                `json:"proposer_validator_id"`
	NumTxs              uint32                `json:"num_txs"              pg:"default:0"`
	BlockTime           uint64                `json:"block_time"`
	CreatedAt           time.Time             `json:"created_at"`
	UpdatedAt           time.Time             `json:"updated_at"`
	BlockReward         string                `json:"block_reward"         pg:"type:numeric(70)"`
	Hash                string                `json:"hash"`
	Proposer            *Validator            `json:"proposer"             pg:"rel:has-one,fk:proposer_validator_id"` //relation has one to Validators
	Validators          []*Validator          `json:"validators"           pg:"many2many:block_validator"`            //relation has many to Validators
	Transactions        []*Transaction        `json:"transactions"         pg:"rel:has-many"`                         //relation has many to Transactions
	InvalidTransactions []*InvalidTransaction `json:"invalid_transactions" pg:"rel:has-many"`                         //relation has many to InvalidTransactions
	Rewards             []*Reward             `json:"rewards"              pg:"rel:has-many"`                         //relation has many to Rewards
	Slashes             []*Slash              `json:"slashes"              pg:"rel:has-many"`                         //relation has many to Slashes
	BlockValidators     []BlockValidator      `json:"block_validators"     pg:"rel:has-many"`
}

//Return block hash with prefix
func (t *Block) GetHash() string {
	return `Mh` + t.Hash
}

type BlockAddresses struct {
	Height    uint64
	Addresses []string
}

type BlockValidators struct {
	Height     uint64
	Validators []*Validator
}
