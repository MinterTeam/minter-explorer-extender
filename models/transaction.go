package models

import (
	"encoding/json"
	"time"
)

const TxTypeSend = 1
const TxTypeSellCoin = 2
const TxTypeSellAllCoin = 3
const TxTypeBuyCoin = 4
const TxTypeCreateCoin = 5
const TxTypeDeclareCandidacy = 6
const TxTypeDelegate = 7
const TxTypeUnbound = 8
const TxTypeRedeemCheck = 9
const TxTypeSetCandidateOnline = 10
const TxTypeSetCandidateOffline = 11
const TxTypeMultiSig = 12
const TxTypeMultiSend = 13
const TxTypeEditCandidate = 14

type Transaction struct {
	ID            uint64               `json:"id"`
	FromAddressID uint64               `json:"from_address_id"`
	Nonce         uint64               `json:"nonce"`
	GasPrice      uint64               `json:"gas_price"`
	Gas           uint64               `json:"gas"`
	BlockID       uint64               `json:"block_id"`
	GasCoinID     uint64               `json:"gas_coin_id"`
	CreatedAt     time.Time            `json:"created_at"`
	Type          uint8                `json:"type"`
	Hash          string               `json:"hash"`
	ServiceData   string               `json:"service_data"`
	Data          json.RawMessage      `json:"data"`
	Tags          map[string]string    `json:"tags"`
	Payload       []byte               `json:"payload"`
	RawTx         []byte               `json:"raw_tx"`
	Block         *Block               `json:"block"`                                             //Relation has one to Blocks
	FromAddress   *Address             `json:"from_address" pg:"fk:from_address_id"`              //Relation has one to Address
	GasCoin       *Coin                `json:"gas_coin"     pg:"fk:gas_coin_id"`                  //Relation has one to Coin
	Validators    []*Validator         `json:"validators"   pg:"many2many:transaction_validator"` //Relation has many to Validators
	TxOutput      []*TransactionOutput `json:"tx_output"`                                         //Relation has one to TransactionOutputs
}

type SendTxData struct {
	Coin  string `json:"coin"`
	To    string `json:"to"`
	Value string `json:"value"`
}

type CreateCoinTxData struct {
	Name                 string `json:"name"`
	Symbol               string `json:"symbol"`
	InitialAmount        string `json:"initial_amount"`
	InitialReserve       string `json:"initial_reserve"`
	ConstantReserveRatio string `json:"constant_reserve_ratio"`
}

type MultiSendTxData []struct {
	Coin    string `json:"coin"`
	Address string `json:"address"`
	Value   string `json:"value"`
}

//Return fee for transaction
func (t *Transaction) GetFee() uint64 {
	return t.GasPrice * t.Gas
}

//Return transactions hash with prefix
func (t *Transaction) GetHash() string {
	return `Mt` + t.Hash
}
