package models

import "github.com/MinterTeam/node-grpc-gateway/api_pb"

type Balance struct {
	AddressID uint     `json:"address_id"  pg:",pk"`
	CoinID    uint     `json:"coin_id"     pg:",pk,use_zero"`
	Value     string   `json:"value"       pg:"type:numeric(70)"`
	Address   *Address `pg:"rel:has-one"`            //Relation has one to Address
	Coin      *Coin    `pg:"rel:has-one,fk:coin_id"` //Relation has one to Coin
}

type BalanceUpdateData struct {
	Block *api_pb.BlockResponse
	Event *api_pb.EventsResponse
}
