package events

import (
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/go-pg/pg/v10"
	"time"
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

func (r *Repository) GetRewardsByDay(now time.Time) ([]*models.AggregatedReward, error) {
	var result []*models.AggregatedReward
	err := r.db.Model(&result).Where("time_id = ?", now.Format("2006-01-02")).Select()
	return result, err
}

func (r *Repository) SaveAggregatedRewards(rewards []*models.AggregatedReward) error {
	_, err := r.db.Model(&rewards).OnConflict("(time_id, address_id, validator_id, role) DO UPDATE").Insert()
	return err
}
