package transaction

import (
	"context"
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
)

type Repository struct {
	db *pg.DB
}

func NewRepository(db *pg.DB) *Repository {
	orm.RegisterTable((*models.TransactionValidator)(nil))
	return &Repository{
		db: db,
	}
}

func (r *Repository) Save(transaction *models.Transaction) error {
	_, err := r.db.Model(transaction).Insert()
	if err != nil {
		return err
	}
	return nil
}

func (r *Repository) SaveAll(transactions []*models.Transaction) error {
	_, err := r.db.Model(&transactions).Insert()
	return err
}

func (r *Repository) SaveAllInvalid(transactions []*models.InvalidTransaction) error {
	_, err := r.db.Model(&transactions).Insert()
	return err
}

func (r *Repository) SaveAllTxOutputs(output []*models.TransactionOutput) error {
	_, err := r.db.Model(&output).Insert()
	return err
}

func (r *Repository) SaveRedeemedChecks(list []*models.Checks) error {
	_, err := r.db.Model(&list).OnConflict("DO NOTHING").Insert()
	return err
}

func (r *Repository) LinkWithValidators(links []*models.TransactionValidator) error {
	_, err := r.db.Model(&links).Insert()
	return err
}

func (r *Repository) IndexTxAddress(txsId []uint64) error {
	_, err := r.db.ExecContext(context.Background(), `
insert into index_transaction_by_address (block_id, address_id, transaction_id)
  (select block_id, from_address_id, id
   from transactions
   where id in (?)
   union
   select t.block_id, to_address_id, transaction_id
   from transaction_outputs
          inner join transactions t on transaction_outputs.transaction_id = t.id
   where t.id in (?))
ON CONFLICT DO NOTHING;
	`, pg.In(txsId), pg.In(txsId))
	return err
}

func (r *Repository) IndexLastNTxAddress(txsNumber int) error {
	_, err := r.db.ExecContext(context.Background(), `
insert into index_transaction_by_address (block_id, address_id, transaction_id)
    (select it.block_id, it.from_address_id, it.id
     from transactions as it
     where it.block_id > (select (id - ?) from blocks order by id desc limit 1)
     union
     select ot.block_id, touts.to_address_id, touts.transaction_id
     from transaction_outputs touts
            inner join transactions ot on touts.transaction_id = ot.id
     where ot.block_id > (select (id - ?) from blocks order by id desc limit 1))
ON CONFLICT DO NOTHING;
	`, txsNumber, txsNumber)
	return err
}
