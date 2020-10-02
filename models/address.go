package models

type Address struct {
	ID                  uint                  `json:"id"      pg:",pk"`
	Address             string                `json:"address" pg:"type:varchar(64)"`
	Balances            []*Balance            `json:"balances"`                                     //relation has many to Balances
	Rewards             []*Reward             `json:"rewards"`                                      //relation has many to Rewards
	Slashes             []*Slash              `json:"slashes"`                                      //relation has many to Slashes
	Transactions        []*Transaction        `json:"transactions" pg:"fk:from_address_id"`         //relation has many to Transactions
	InvalidTransactions []*InvalidTransaction `json:"invalid_transactions" pg:"fk:from_address_id"` //relation has many to InvalidTransactions
}

// Return address with prefix
func (a *Address) GetAddress() string {
	return `Mx` + a.Address
}
