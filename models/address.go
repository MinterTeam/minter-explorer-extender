package models

type Address struct {
	ID      uint   `json:"id"      pg:",pk"`
	Address string `json:"address" pg:",unique; type:varchar(64)"`
}
