package events

import (
	"github.com/MinterTeam/minter-explorer-tools/models"
	"github.com/go-pg/pg"
)

type Repository struct {
	db *pg.DB
}

func NewRepository(db *pg.DB) *Repository {
	return &Repository{
		db: db,
	}
}

func (r *Repository) SaveRewards(rewards []*models.Reward) error {
	var args []interface{}
	for _, reward := range rewards {
		args = append(args, reward)
	}
	return r.db.Insert(args...)
}

func (r *Repository) SaveSlashes(slashes []*models.Slash) error {
	var args []interface{}
	for _, slash := range slashes {
		args = append(args, slash)
	}
	return r.db.Insert(args...)
}

func (r *Repository) AggregateRewards(aggregateInterval string, beforeBlockId uint64) error {
	if aggregateInterval != "hour" && aggregateInterval != "day" {
		return errors.New("Not acceptable aggregate interval")
	}
	_, err := r.db.Query(nil, `
insert into aggregated_rewards (time_id,
                                from_block_id,
                                to_block_id,
                                address_id,
                                validator_id,
                                role,
                                amount) (select date_trunc('?', b.created_at) as time_id,
                                                min(r.block_id)               as from_block_id,
                                                max(r.block_id)               as to_block_id,
                                                r.address_id,
                                                r.validator_id,
                                                r.role,
                                                sum(r.amount)                 as amount
                                         from rewards r
                                                inner join blocks b on r.block_id = b.id
                                         where r.block_id >=
                                               (select coalesce(max(from_block_id), 1) from aggregated_rewards)
                                           AND r.block_id < ?
    group by r.address_id, r.validator_id, r.role, date_trunc('?', b.created_at)
    order by min (r.block_id) desc)
ON CONFLICT (from_block_id,address_id,validator_id,role)
            DO UPDATE set amount = EXCLUDED.amount, to_block_id = EXCLUDED.to_block_id;
	`, aggregateInterval, beforeBlockId, aggregateInterval)
	return err
}
