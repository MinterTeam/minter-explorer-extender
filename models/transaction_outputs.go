package models

type TransactionOutput struct {
	ID            uint64       `json:"id"`
	TransactionID uint64       `json:"transaction_id"`
	ToAddressID   uint64       `json:"to_address_id"`
	CoinID        uint         `json:"coin_id"     pg:",use_zero"`
	Value         string       `json:"value"       pg:"type:numeric(70)"`
	Coin          *Coin        `json:"coin"        pg:"rel:has-one"`                  //Relation has one to Coins
	ToAddress     *Address     `json:"to_address"  pg:"rel:has-one,fk:to_address_id"` //Relation has one to Addresses
	Transaction   *Transaction `json:"transaction" pg:"rel:has-one"`                  //Relation has one to Transactions
}
