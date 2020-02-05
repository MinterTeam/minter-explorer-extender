package block

import (
	"github.com/MinterTeam/minter-explorer-tools/v4/models"
	"github.com/go-pg/pg/v9"
)

type Repository struct {
	db *pg.DB
}

func NewRepository(db *pg.DB) *Repository {
	return &Repository{
		db: db,
	}
}

func (r *Repository) Save(block *models.Block) error {
	_, err := r.db.Model(block).Insert()
	if err != nil {
		return err
	}
	return nil
}

func (r *Repository) GetLastFromDB() (*models.Block, error) {
	block := new(models.Block)
	err := r.db.Model(block).Last()
	if err != nil {
		return nil, err
	}
	return block, nil
}

func (r *Repository) LinkWithValidators(links []*models.BlockValidator) error {
	var args []interface{}
	for _, l := range links {
		args = append(args, l)
	}
	err := r.db.Insert(args...)
	return err
}

func (r *Repository) DeleteLastBlockData() error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	// Rollback tx on error.
	defer tx.Rollback()
	_, err = tx.Query(nil, `delete from transaction_outputs where transaction_id IN (select distinct id from transactions where block_id = (select id from blocks order by id desc limit 1));`)
	_, err = tx.Query(nil, `delete from transaction_validator where transaction_id IN (select distinct id from transactions where block_id = (select id from blocks order by id desc limit 1));`)
	_, err = tx.Query(nil, `delete from index_transaction_by_address where transaction_id in (select distinct id from transactions where block_id = (select id from blocks order by id desc limit 1));`)
	_, err = tx.Query(nil, `delete from invalid_transactions  where block_id = (select id from blocks order by id desc limit 1);`)
	_, err = tx.Query(nil, `delete from transactions where block_id = (select id from blocks order by id desc limit 1);`)
	_, err = tx.Query(nil, `delete from rewards where block_id = (select id from blocks order by id desc limit 1);`)
	_, err = tx.Query(nil, `delete from slashes where block_id = (select id from blocks order by id desc limit 1);`)
	_, err = tx.Query(nil, `delete from block_validator where block_id = (select id from blocks order by id desc limit 1);`)
	_, err = tx.Query(nil, `delete from blocks where id = (select id from blocks order by id desc limit 1);`)
	return tx.Commit()
}
