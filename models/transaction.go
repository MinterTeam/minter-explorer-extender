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
	TxOutputs     []*TransactionOutput `json:"tx_outputs"`
	TxOutput      *TransactionOutput   `json:"tx_output"`
}

type TransactionValidator struct {
	tableName     struct{} `sql:"transaction_validator"`
	TransactionID uint64
	ValidatorID   uint64
}

type SendTxData struct {
	Coin  string `json:"coin"`
	To    string `json:"to"`
	Value string `json:"value"`
}

type SellCoinTxData struct {
	CoinToSell        string `json:"coin_to_sell"`
	ValueToSell       string `json:"value_to_sell"`
	CoinToBuy         string `json:"coin_to_buy"`
	MinimumValueToBuy string `json:"minimum_value_to_buy"`
}

type SellAllCoinTxData struct {
	CoinToSell        string `json:"coin_to_sell"`
	CoinToBuy         string `json:"coin_to_buy"`
	MinimumValueToBuy string `json:"minimum_value_to_buy"`
}

type BuyCoinTxData struct {
	CoinToBuy          string `json:"coin_to_buy"`
	ValueToBuy         string `json:"value_to_buy"`
	CoinToSell         string `json:"coin_to_sell"`
	MaximumValueToSell string `json:"maximum_value_to_sell"`
}

type CreateCoinTxData struct {
	Name                 string `json:"name"`
	Symbol               string `json:"symbol"`
	InitialAmount        string `json:"initial_amount"`
	InitialReserve       string `json:"initial_reserve"`
	ConstantReserveRatio string `json:"constant_reserve_ratio"`
}

type DeclareCandidacyTxData struct {
	Address    string `json:"address"`
	PubKey     string `json:"pub_key"`
	Commission string `json:"commission"`
	Coin       string `json:"coin"`
	Stake      string `json:"stake"`
}

type DelegateTxData struct {
	PubKey string `json:"pub_key"`
	Coin   string `json:"coin"`
	Stake  string `json:"stake"`
}

type UnbondTxData struct {
	PubKey string `json:"pub_key"`
	Coin   string `json:"coin"`
	Value  string `json:"value"`
}

type RedeemCheckTxData struct {
	RawCheck string `json:"raw_check"`
	Proof    string `json:"proof"`
}

type SetCandidateTxData struct {
	PubKey string `json:"pub_key"`
}

type EditCandidateTxData struct {
	PubKey        string `json:"pub_key"`
	RewardAddress string `json:"reward_address"`
	OwnerAddress  string `json:"owner_address"`
}

type CreateMultisigTxData struct {
	Threshold string   `json:"threshold"`
	Weights   []uint   `json:"weights"`
	Addresses []string `json:"addresses"`
}

type MultiSendTxData struct {
	List []SendTxData `json:"list"`
}

//Return fee for transaction
func (t *Transaction) GetFee() uint64 {
	return t.GasPrice * t.Gas
}

//Return transactions hash with prefix
func (t *Transaction) GetHash() string {
	return `Mt` + t.Hash
}
