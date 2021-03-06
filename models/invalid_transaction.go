package models

import "time"

type InvalidTransaction struct {
	ID            uint64    `json:"id" pg:",pk"`
	FromAddressID uint64    `json:"from_address_id"`
	BlockID       uint64    `json:"block_id"`
	CreatedAt     time.Time `json:"created_at"`
	Type          uint8     `json:"type"`
	Hash          string    `json:"hash"`
	TxData        string    `json:"tx_data"`
	Log           string    `json:"log"`
	Block         *Block    `pg:"rel:has-one"`                    //Relation has one to Blocks
	FromAddress   *Address  `pg:"rel:has-one,fk:from_address_id"` //Relation has one to Addresses
}

// GetHash Return transactions hash with prefix
func (t InvalidTransaction) GetHash() string {
	return `Mt` + t.Hash
}
