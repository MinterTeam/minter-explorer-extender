package models

type Address struct {
	ID                  uint                  `json:"id"                   pg:",pk"`
	Address             string                `json:"address"              pg:"type:varchar(64)"`
	Balances            []*Balance            `json:"balances"             pg:"rel:has-many"`                    //relation has many to Balances
	Rewards             []*Reward             `json:"rewards"              pg:"rel:has-many"`                    //relation has many to Rewards
	Slashes             []*Slash              `json:"slashes"              pg:"rel:has-many"`                    //relation has many to Slashes
	Transactions        []*Transaction        `json:"transactions"         pg:"rel:has-many,fk:from_address_id"` //relation has many to Transactions
	InvalidTransactions []*InvalidTransaction `json:"invalid_transactions" pg:"rel:has-many,fk:from_address_id"` //relation has many to InvalidTransactions
}

// Return address with prefix
func (a *Address) GetAddress() string {
	return `Mx` + a.Address
}
