package models

import (
	"time"
)

const ValidatorStatusNotReady = 1
const ValidatorStatusReady = 2

type Validator struct {
	ID                   uint                  `json:"id" pg:",pk"`
	RewardAddressID      *uint                 `json:"reward_address_id"`
	OwnerAddressID       *uint                 `json:"owner_address_id"`
	ControlAddressID     *uint                 `json:"control_address_id"`
	CreatedAtBlockID     *uint                 `json:"created_at_block_id"`
	PublicKey            string                `json:"public_key"  pg:"type:varchar(64)"`
	Status               *uint8                `json:"status"`
	Commission           *uint64               `json:"commission"`
	TotalStake           *string               `json:"total_stake"   pg:"type:numeric(70)"`
	Name                 *string               `json:"name"`
	SiteUrl              *string               `json:"site_url"`
	IconUrl              *string               `json:"icon_url"`
	Description          *string               `json:"description"`
	MetaUpdatedAtBlockID *uint64               `json:"meta_updated_at_block_id"`
	BanedTill            uint64                `json:"baned_till"`
	UpdateAt             *time.Time            `json:"update_at"`
	ControlAddress       *Address              `json:"control_address" pg:"rel:has-one,fk:control_address_id"`
	RewardAddress        *Address              `json:"reward_address"  pg:"rel:has-one,fk:reward_address_id"`
	OwnerAddress         *Address              `json:"owner_address"   pg:"rel:has-one,fk:owner_address_id"`
	Stakes               []*Stake              `json:"stakes"          pg:"rel:has-many"`
	PublicKeys           []ValidatorPublicKeys `json:"public_keys"     pg:"rel:has-many"`
}

//Return validators PK with prefix
func (v Validator) GetPublicKey() string {
	return `Mp` + v.PublicKey
}
