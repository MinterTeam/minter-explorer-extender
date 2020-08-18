package models

import "time"

type ValidatorPublicKeys struct {
	ID          uint       `json:"id"  pg:",pk"`
	ValidatorId uint       `json:"validator_id"`
	Key         string     `json:"key" pg:"type:varchar(64)"`
	CreatedAt   *time.Time `json:"created_at"`
	UpdateAt    *time.Time `json:"update_at"`
}
