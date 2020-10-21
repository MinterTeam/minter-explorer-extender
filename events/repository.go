package events

import (
	"context"
	"errors"
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/go-pg/pg/v10"
	"strings"
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
	_, err := r.db.Model(&rewards).Insert()
	return err
}

func (r *Repository) SaveSlashes(slashes []*models.Slash) error {
	_, err := r.db.Model(&slashes).Insert()
	return err
}

func (r *Repository) AggregateRewards(aggregateInterval string, beforeBlockId uint64) error {

	if strings.Compare(aggregateInterval, "hour") != 0 && strings.Compare(aggregateInterval, "day") != 0 {
		return errors.New("not acceptable aggregate interval")
	}

	_, err := r.db.ExecContext(context.Background(), `
insert into aggregated_rewards (time_id,
                                from_block_id,
                                to_block_id,
                                address_id,
                                validator_id,
                                role,
                                amount) (select date_trunc(?, b.created_at) as time_id,
                                                min(r.block_id)                  as from_block_id,
                                                max(r.block_id)                  as to_block_id,
                                                r.address_id,
                                                r.validator_id,
                                                r.role,
                                                sum(r.amount)                    as amount
                                         from rewards r
                                                inner join blocks b on r.block_id = b.id
                                         where b.created_at >=
                                               (select coalesce(max(time_id), (select min(created_at) from blocks))
                                                from aggregated_rewards)
                                           and b.created_at < (select created_at from blocks where id = ?)
                                         group by r.address_id, r.validator_id, r.role, date_trunc(?, b.created_at)
                                         order by min(b.created_at) desc)
ON CONFLICT (time_id,address_id,validator_id,role)
            DO UPDATE set amount = EXCLUDED.amount, to_block_id = EXCLUDED.to_block_id, from_block_id = EXCLUDED.from_block_id;
	`, aggregateInterval, beforeBlockId, aggregateInterval)
	return err
}

func (r *Repository) DropOldRewardsData(saveBlocksCount uint32) error {
	_, err := r.db.ExecContext(context.Background(), `
		delete from rewards where block_id < ((select id from blocks order by id desc limit 1) - ?);
	`, saveBlocksCount)
	return err
}
