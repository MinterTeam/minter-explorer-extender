package models

type ExtenderEnvironment struct {
	AppName         string
	Debug           bool
	DbName          string
	DbUser          string
	DbPassword      string
	DbMinIdleConns  int
	DbPoolSize      int
	NodeApi         string
	ApiHost         string
	ApiPort         int
	TxChunkSize     int
	EventsChunkSize int
}
