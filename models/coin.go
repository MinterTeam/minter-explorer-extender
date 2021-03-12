package models

import (
	"fmt"
	"time"
)

type CoinType byte

const (
	_ CoinType = iota
	CoinTypeBase
	CoinTypeToken
	CoinTypePoolToken
)

type Coin struct {
	ID               uint       `json:"id" pg:",use_zero"`
	Type             CoinType   `json:"type"`
	Name             string     `json:"name"`
	Symbol           string     `json:"symbol"`
	Volume           string     `json:"volume"     pg:"type:numeric(70)"`
	Crr              uint       `json:"crr"`
	Reserve          string     `json:"reserve"    pg:"type:numeric(70)"`
	MaxSupply        string     `json:"max_supply" pg:"type:numeric(70)"`
	Version          uint       `json:"version"    pg:",use_zero"`
	OwnerAddressId   uint       `json:"owner_address"`
	CreatedAtBlockId uint       `json:"created_at_block_id"`
	Burnable         bool       `json:"burnable"`
	Mintable         bool       `json:"mintable"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        *time.Time `json:"updated_at"`
	DeletedAt        *time.Time `pg:",soft_delete"`
	OwnerAddress     Address    `pg:"rel:has-one,rel:has-one"`
}

// Return coin with version
func (c *Coin) GetSymbol() string {
	if c.Version == 0 {
		return c.Symbol
	}

	return fmt.Sprintf("%s-%d", c.Symbol, c.Version)
}
