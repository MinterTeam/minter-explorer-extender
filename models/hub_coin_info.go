package models

type HubCoinsInfoResponse struct {
	List struct {
		Tokens []TokenInfo `json:"token_infos"`
	} `json:"list"`
}

type TokenInfo struct {
	Id               string `json:"id"`
	Denom            string `json:"denom"`
	ChainId          string `json:"chain_id"`
	ExternalTokenId  string `json:"external_token_id"`
	ExternalDecimals string `json:"external_decimals"`
	Commission       string `json:"commission"`
}

type TokenContract struct {
	CoinId uint64 `json:"coin_id" pg:",pk,use_zero"`
	Eth    string `json:"eth"`
	Bsc    string `json:"bsc"`
}
