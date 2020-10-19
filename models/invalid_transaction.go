package models

import "time"

type InvalidTransaction struct {
	ID            uint64    `json:"id" pg:",pk"`
	FromAddressID uint64    `json:"from_address_id"`
	BlockID       uint64    `json:"block_id"`
	CreatedAt     time.Time `json:"created_at"`
	Type          uint8     `json:"type"`
	Hash          string    `json:"hash"`
	TxData        string    `json:"tx_data" pg:",jsonb"`
	Block         *Block    //Relation has one to Blocks
	FromAddress   *Address  `pg:"fk:from_address_id"` //Relation has one to Addresses
}

//Return transactions hash with prefix
func (t InvalidTransaction) GetHash() string {
	return `Mt` + t.Hash
}
