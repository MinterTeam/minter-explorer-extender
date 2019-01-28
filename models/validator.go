package models

import "time"

type Validator struct {
	ID               uint64    `json:"id"`
	RewardAddressID  uint64    `json:"reward_address_id"`
	OwnerAddressID   uint64    `json:"owner_address_id"`
	CreatedAtBlockID uint64    `json:"created_at_block_id"`
	Status           uint8     `json:"status"`
	Commission       uint64    `json:"commission"`
	TotalStake       string    `json:"total_stake"`
	PublicKey        string    `json:"public_key" sql:"type:varchar(64)"`
	UpdateAt         time.Time `json:"update_at"`
	RewardAddress    *Address  `json:"reward_address" pg:"fk:reward_address_id"`
	OwnerAddress     *Address  `json:"owner_address"  pg:"fk:owner_address_id"`
}

func (v Validator) GetPublicKey() string {
	return `Mp` + v.PublicKey
}
