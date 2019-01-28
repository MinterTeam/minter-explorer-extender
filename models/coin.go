package models

import "time"

type Coin struct {
	ID                    uint64    `json:"id"`
	CreationAddressID     uint64    `json:"creation_address_id"`
	CreationTransactionID uint64    `json:"creation_transaction_id"`
	DeletedAtBlockID      uint64    `json:"deleted_at_block_id"`
	Crr                   uint64    `json:"crr"`
	UpdatedAt             time.Time `json:"updated_at"`
	Volume                string    `json:"volume"`
	ReserveBalance        string    `json:"reserve_balance"`
	Name                  string    `json:"name" sql:"type:varchar(255)"`
	Symbol                string    `json:"symbol" sql:"type:varchar(20)"`
}
