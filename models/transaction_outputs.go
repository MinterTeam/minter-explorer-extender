package models

type TransactionOutput struct {
	ID            uint64       `json:"id"`
	TransactionID uint64       `json:"transaction_id"`
	ToAddressID   uint64       `json:"to_address_id"`
	CoinID        uint64       `json:"coin_id"`
	Value         string       `json:"value" sql:"type:numeric(70)"`
	Coin          *Coin        `json:"coin"`                             //Relation has one to Coins
	ToAddress     *Address     `json:"to_address" pg:"fk:to_address_id"` //Relation has one to Addresses
	Transaction   *Transaction `json:"transaction"`                      //Relation has one to Transactions
}
