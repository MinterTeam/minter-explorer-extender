package models

type ExtenderEnvironment struct {
	AppName                  string
	Debug                    bool
	DbName                   string
	DbUser                   string
	DbPassword               string
	DbMinIdleConns           int
	DbPoolSize               int
	NodeApi                  string
	ApiHost                  string
	ApiPort                  int
	TxChunkSize              int
	EventsChunkSize          int
	WrkSaveRewardsCount      int
	WrkSaveSlashesCount      int
	WrkSaveTxsCount          int
	WrkSaveTxsOutputCount    int
	WrkSaveInvTxsCount       int
	WrkSaveAddressesCount    int
	WrkSaveValidatorTxsCount int
}
