package models

type ExtenderEnvironment struct {
	Debug       bool
	DbName      string
	DbUser      string
	DbPassword  string
	NodeApi     string
	WsLink      string
	WsKey       string
	ApiHost     string
	ApiPort     int
	TxChunkSize int
}
