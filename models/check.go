package models

type Checks struct {
	TransactionID uint64 `json:"transaction_id"`
	Data          string `json:"data"`
	FromAddressId uint   `json:"from_address_id"`
	ToAddressId   uint   `json:"to_address_id"`
}
