package models

import "time"

type Transaction struct {
	ID            uint64             `json:"id"`
	FromAddressID uint64             `json:"from_address_id"`
	Nonce         uint64             `json:"nonce"`
	GasPrice      uint64             `json:"gas_price"`
	Gas           uint64             `json:"gas"`
	BlockID       uint64             `json:"block_id"`
	GasCoinID     uint64             `json:"gas_coin_id"`
	CreatedAt     time.Time          `json:"created_at"`
	Type          uint8              `json:"type"`
	Hash          string             `json:"hash"`
	ServiceData   string             `json:"service_data"`
	Data          map[string]string  `json:"data"` //Relation has one to Address
	Tags          map[string]string  `json:"tags"`
	Payload       []byte             `json:"payload"`
	RawTx         []byte             `json:"raw_tx"`
	Block         *Block             `json:"block"`                                             //Relation has one to Blocks
	FromAddress   *Address           `json:"from_address" pg:"fk:from_address_id"`              //Relation has one to Address
	GasCoin       *Coin              `json:"gas_coin"     pg:"fk:gas_coin_id"`                  //Relation has one to Coin
	Validators    []*Validator       `json:"validators"   pg:"many2many:transaction_validator"` //Relation has many to Validators
	TxOutput      *TransactionOutput `json:"tx_output"`                                         //Relation has one to TransactionOutputs
}

//Return fee for transaction
func (t *Transaction) GetFee() uint64 {
	return t.GasPrice * t.Gas
}

//Return transactions hash with prefix
func (t *Transaction) GetHash() string {
	return `Mh` + t.Hash
}
