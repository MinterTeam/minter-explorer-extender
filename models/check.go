package models

type Check struct {
	TransactionID uint64   `json:"transaction_id"`
	Data          string   `json:"data"`
	FromAddressId uint     `json:"from_address_id"`
	ToAddressId   uint     `json:"to_address_id"`
	FromAddress   *Address `json:"from_address" pg:"fk:from_address_id"`
	ToAddress     *Address `json:"to_address"   pg:"fk:to_address_id"`
}
