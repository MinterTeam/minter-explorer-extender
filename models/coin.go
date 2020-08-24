package models

import "time"

type Coin struct {
	ID             uint       `json:"id" pg:",use_zero"`
	Name           string     `json:"name"`
	Symbol         string     `json:"symbol"`
	Volume         string     `json:"volume"`
	Crr            uint       `json:"crr"`
	Reserve        string     `json:"reserve"`
	MaxSupply      string     `json:"max_supply"`
	Version        uint       `json:"version"  pg:",use_zero"`
	OwnerAddressId uint       `json:"owner_address"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      *time.Time `json:"updated_at"`
	DeletedAt      *time.Time `pg:",soft_delete"`
	OwnerAddress   Address    `pg:"fk:id"`
}
