package models

import "time"

type Address struct {
	ID        uint64    `json:"id"`
	Address   string    `json:"address" sql:",unique; type:varchar(64)"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (a *Address) GetAddress() string {
	return `Mx` + a.Address
}
