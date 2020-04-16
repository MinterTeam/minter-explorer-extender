package core

import (
	"context"
	"fmt"
	"github.com/MinterTeam/minter-explorer-extender/v2/env"
	"github.com/MinterTeam/minter-explorer-tools/v4/models"
	"github.com/gin-gonic/gin"
	"github.com/go-pg/pg/v9"
	"github.com/joho/godotenv"
	"log"
	"net/http"
	"strings"
	"testing"
)

type TestKit struct {
	fakeNode *http.Server
	db       *pg.DB
}

var envData *env.ExtenderEnvironment
var testKit *TestKit

func init() {
	err := godotenv.Load("../.env")
	if err != nil {
		log.Fatal(err)
	}

	envData = env.New("test")

	err = prepareDB(envData)
	if err != nil {
		log.Fatal(err)
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.GET("/block", fakeBlock)
	router.GET("/status", fakeStatus)
	router.GET("/events", fakeEvents)
	router.GET("/addresses", fakeAddresses)
	router.GET("/coin_info", fakeCoinInfo)
	router.GET("/candidates", fakeCandidates)

	testKit = &TestKit{
		fakeNode: &http.Server{
			Addr:    ":9900",
			Handler: router,
		},
		db: pg.Connect(&pg.Options{
			Addr:     fmt.Sprintf("%s:%s", envData.DbHost, envData.DbPort),
			User:     envData.DbUser,
			Password: envData.DbPassword,
			Database: envData.DbName,
		}),
	}

	go func() {
		if err := testKit.fakeNode.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	ext := NewExtender(envData)
	ext.Run(1)
}

func TestExtender_Run(t *testing.T) {

	testBlock(t, testKit.db)
	testBalance(t, testKit.db)
	testTransactions(t, testKit.db)
	testValidators(t, testKit.db)
	testEvents(t, testKit.db)

	if err := testKit.fakeNode.Shutdown(context.Background()); err != nil {
		t.Error(err)
	}
	t.Log("Server has been shutdown")
}

func testEvents(t *testing.T, db *pg.DB) {
	t.Log("Testing events...")

	var rewards []models.Reward
	err := db.Model(&rewards).Select()

	if err != nil {
		t.Error("Test Events: ", err)
		return
	}

	if len(rewards) < 1 {
		t.Error("Test Events: Wrong rewards count, expect more than 1 got", len(rewards))
		return
	}

	errorCount := 0
	for _, r := range rewards {
		if r.BlockID != 1 {
			errorCount++
			t.Error("Test Events: Wrong reward block id")
		}
		if r.AddressID != 1 {
			t.Error("Test Events: Wrong reward address id")
			errorCount++
		}
		if r.ValidatorID != 1 {
			t.Error("Test Events: Wrong reward validator id")
			errorCount++
		}
		if r.Amount != "10000000000000000" {
			t.Error("Test Events: Wrong reward amount")
			errorCount++
		}
		if r.Role != "DAO" {
			t.Error("Test Events: Wrong reward role")
			errorCount++
		}
	}
	if errorCount == 0 {
		t.Log("Rewards are OK")
	}

	var slashes []models.Slash
	err = db.Model(&slashes).Select()

	_, err = db.Model(&slashes).Order("id ASC").SelectAndCount()
	if err != nil {
		t.Error("Test Slashes: ", err)
		return
	}
	if len(slashes) < 1 {
		t.Error("Test Events: Wrong slashes count, expect more than 1 got", len(slashes))
		return
	}

	errorCount = 0
	for _, s := range slashes {
		if s.BlockID != 1 {
			errorCount++
			t.Error("Test Events: Wrong slash block id")
		}
		if s.AddressID != 1 {
			t.Error("Test Events: Wrong slash address id")
			errorCount++
		}
		if s.ValidatorID != 1 {
			t.Error("Test Events: Wrong slash validator id")
			errorCount++
		}
		if s.Amount != "10000000000000000" {
			t.Error("Test Events: Wrong slash reward amount")
			errorCount++
		}
		if s.CoinID != 1 {
			t.Error("Test Events: Wrong slash coin id")
			errorCount++
		}
	}
	if errorCount == 0 {
		t.Log("Slashes are OK")
	}
}

func testValidators(t *testing.T, db *pg.DB) {
	t.Log("Testing validator...")
	v := new(models.Validator)
	errorCount := 0

	err := db.Model(v).Where("id = ?", 1).Select()
	if err != nil {
		t.Error(err)
	}

	if *v.RewardAddressID != 1 {
		errorCount++
		t.Error("Test validator: Wrong reward address ID")
	}
	if *v.OwnerAddressID != 1 {
		errorCount++
		t.Error("Test validator: Wrong owner address ID")
	}
	if *v.Status != 2 {
		errorCount++
		t.Error("Test validator: Wrong status")
	}
	if *v.Commission != 5 {
		errorCount++
		t.Error("Test validator: Wrong commission")
	}
	if *v.TotalStake != "1111111111111111111111111111111111111111" {
		errorCount++
		t.Error("Test validator: Wrong total stake")
	}
	if v.PublicKey != "fe176f944623a8ca9f409a62f0ea3ca75c1cf8d89970adf9384fc9ae8d77fa0b" {
		errorCount++
		t.Error("Test validator: Wrong public key")
	}
	if errorCount == 0 {
		t.Log("Validator is OK")
	}

}

func testBlock(t *testing.T, db *pg.DB) {
	t.Log("Testing block...")
	errorCount := 0
	b := new(models.Block)
	err := db.Model(b).Where("id = ?", 1).Select()
	if err != nil {
		t.Error(err)
	}

	if b.ID != 1 {
		errorCount++
		t.Error("Test Block: Wrong ID")
	}
	if b.Size != 4542 {
		errorCount++
		t.Error("Test Block: Wrong block size")
	}
	if b.ProposerValidatorID != 1 {
		errorCount++
		t.Error("Test Block: Wrong proposer ID")
	}
	if b.NumTxs != 14 {
		errorCount++
		t.Error("Test Block: Wrong transaction count")
	}
	if b.BlockTime != 1000000000 {
		errorCount++
		t.Error("Test Block: Wrong block time")
	}
	if b.BlockReward != "308000000000000000000" {
		errorCount++
		t.Error("Test Block: Wrong block reward")
	}
	if b.Hash != "05543f3323f75c244cd871e0fd58d356c60a1d82551431c646f4ac1f90f27a45" {
		errorCount++
		t.Error("Test Block: Wrong block hash")
	}

	if errorCount == 0 {
		t.Log("Block is OK")
	}
}

func testBalance(t *testing.T, db *pg.DB) {
	t.Log("Testing balances...")

	var balances []models.Balance
	_, err := db.Model(&balances).Order("id ASC").SelectAndCount()

	if err != nil {
		t.Error("Test Balance: ", err)
		return
	}
	if len(balances) != 6 {
		t.Error("Test Balance: Wrong balances count, expect 6 got", len(balances))
		return
	}

	t.Log("Balances are OK")
}

func testTransactions(t *testing.T, db *pg.DB) {
	t.Log("Testing transactions...")

	var txs []models.Transaction
	_, err := db.Model(&txs).Order("id ASC").SelectAndCount()

	if err != nil {
		t.Error("Test transactions: ", err)
		return
	}
	if len(txs) != 14 {
		t.Error("Test transactions: Wrong transactions count, expect 14 got", len(txs))
		return
	}

	errorCount := 0

	for _, tx := range txs {
		if tx.FromAddressID != 1 {
			errorCount++
			t.Error("Test transaction: Wrong from address id")
		}
		if tx.BlockID != 1 {
			errorCount++
			t.Error("Test transaction: Wrong block id")
		}
		if tx.GasCoinID != 1 {
			errorCount++
			t.Error("Test transaction: Wrong gas coin id")
		}
		if tx.Nonce != uint64(tx.Type) {
			errorCount++
			t.Error("Test transaction: Wrong nonce")
		}
		if tx.GasPrice != uint64(tx.Type) {
			errorCount++
			t.Error("Test transaction: Wrong gas price")
		}
		if tx.Gas != uint64(tx.Type) {
			errorCount++
			t.Error("Test transaction: Wrong gas")
		}
		if tx.Hash == "" {
			errorCount++
			t.Error("Test transaction: Wrong hash")
		}
		if tx.Data == nil {
			errorCount++
			t.Error("Test transaction: Wrong transaction data")
		}
		if tx.Tags == nil {
			errorCount++
			t.Error("Test transaction: Wrong transaction tags")
		}
		if tx.RawTx == nil {
			errorCount++
			t.Error("Test transaction: Wrong raw transaction")
		}
	}

	if errorCount == 0 {
		t.Log("Transactions are OK")
	}
}

func prepareDB(env *env.ExtenderEnvironment) error {
	tables := []string{
		"addresses",
		"aggregated_rewards",
		"balances",
		"block_validator",
		"blocks",
		"coins",
		"index_transaction_by_address",
		"invalid_transactions",
		"rewards",
		"slashes",
		"stakes",
		"transaction_outputs",
		"transaction_validator",
		"transactions",
		"validators",
	}

	//Init DB
	db := pg.Connect(&pg.Options{
		Addr:     fmt.Sprintf("%s:%s", env.DbHost, env.DbPort),
		User:     env.DbUser,
		Password: env.DbPassword,
		Database: env.DbName,
	})

	_, err := db.Query(nil, `truncate `+strings.Join(tables, ", ")+` restart identity cascade`)
	if err != nil {
		return err
	}

	fmt.Println("DB has been truncated")

	coins := []*models.Coin{
		{
			Crr:            100,
			MaxSupply:      "",
			Volume:         "",
			ReserveBalance: "",
			Name:           "BIP",
			Symbol:         "BIP",
		},
		{
			Crr:            100,
			MaxSupply:      "",
			Volume:         "",
			ReserveBalance: "",
			Name:           "MNT",
			Symbol:         "MNT",
		},
	}
	err = db.Insert(&coins)
	fmt.Println("Base coin has been added")

	addresses := []*models.Address{
		{
			Address: "1111111111111111111111111111111111111111",
		},
		{
			Address: "2222222222222222222222222222222222222222",
		},
		{
			Address: "df1050b032d6feb23f00e65522820fa94838401b",
		},
	}
	err = db.Insert(&addresses)
	fmt.Println("Base addresses has been added")

	return err
}

func fakeBlock(c *gin.Context) {
	resp := `
{
  "jsonrpc": "2.0",
  "id": "",
  "result": {
    "hash": "05543f3323f75c244cd871e0fd58d356c60a1d82551431c646f4ac1f90f27a45",
    "height": "1",
    "time": "2020-03-06T08:02:11.997331795Z",
    "num_txs": "14",
    "transactions": [
      {
        "hash": "Mte0876c635448a97163a5104eab27c96cc58b09804016a588cdad8977cb4de5da",
        "raw_tx": "f887820dec01018a4249500000000000000001abea8a4249500000000000000094dd587dad689a72b1be371932f52b6dc8cbfc584a8901f5207dae575b0000808001b845f8431ba04fd4870dbc8b1d80628697c036cb74fb5520746be90ffe41967f8ff53bb2a7e3a02f8de3e3fb05d49447e53f7df2d38861202bec536613b8986195f16fd074863b",
        "from": "Mx1111111111111111111111111111111111111111",
        "nonce": "1",
        "gas_price": 1,
        "type": 1,
        "data": {
          "coin": "BIP",
          "to": "Mx2222222222222222222222222222222222222222",
          "value": "36110000000000000000"
        },
        "payload": "",
        "service_data": "",
        "gas": "1",
        "gas_coin": "BIP",
        "tags": {
          "tx.coin": "BIP",
          "tx.type": "01",
          "tx.from": "1111111111111111111111111111111111111111",
          "tx.to": "2222222222222222222222222222222222222222"
        }
      },
      {
        "hash": "Mtcbcb96638472484c782bf28061b5b4586b7cfb84624fdb3a2becd1326ff44ef5",
        "raw_tx": "f87d820c8301018a4249500000000000000002a1e08a4e4f54524144494e4700880de0b6b3a76400008a4249500000000000000080808001b845f8431ca0dac6ff76e2e2a22b028392a490c9479d1f63831db1b6fe562d51153019f6554ea0047ee7bece6e801a8a15326f088ae45f078d3b5a909c56a1c3fd85cfee77287b",
        "from": "Mx1111111111111111111111111111111111111111",
        "nonce": "2",
        "gas_price": 2,
        "type": 2,
        "data": {
          "coin_to_sell": "MNT",
          "value_to_sell": "1000000000000000000",
          "coin_to_buy": "BIP",
          "minimum_value_to_buy": "0"
        },
        "payload": "",
        "service_data": "",
        "gas": "2",
        "gas_coin": "BIP",
        "tags": {
          "tx.type": "02",
          "tx.from": "1111111111111111111111111111111111111111",
          "tx.coin_to_buy": "BIP",
          "tx.coin_to_sell": "MNT",
          "tx.return": "35007244132974859772819"
        }
      },
      {
        "hash": "Mt7f30816aa699127fbfbc6c97f42e7218ba44137fbbdad81d7cc85327d33cfa28",
        "raw_tx": "f87c8263dd01018a4249500000000000000003a0df8a424950000000000000008a47414d455a5a0000000088071cd076509711b0808001b845f8431ba02e07ced23bf2095db4bcbb6ff9c070aeffb81ba8e663f9624aca18944222be06a05acde5081288191d287d2dda4e410d076d4f4fb5004c801ea0901283aa4736d4",
        "from": "Mx1111111111111111111111111111111111111111",
        "nonce": "3",
        "gas_price": 3,
        "type": 3,
        "data": {
          "coin_to_sell": "BIP",
          "coin_to_buy": "MNT",
          "minimum_value_to_buy": "512513664190190000"
        },
        "payload": "",
        "service_data": "",
        "gas": "3",
        "gas_coin": "BIP",
        "tags": {
          "tx.type": "03",
          "tx.from": "1111111111111111111111111111111111111111",
          "tx.coin_to_buy": "MNT",
          "tx.coin_to_sell": "BIP",
          "tx.return": "574377373906779692",
          "tx.sell_amount": "8455764995458473436"
        }
      },
      {
        "hash": "Mt95507d0f2961eae300bfb3eff1ae937c69fb5aef41544870a76c6a9a1095c5c2",
        "raw_tx": "f88e82012801018a4249500000000000000004b2f18a424950000000000000008a043c33c19375648000008a53434343414d4d4d4d008f01bc16d674ec7ff21f494c589c0000808001b845f8431ca075e5a85ab2e8a2e6cdec068a0241edd583443c04e30a489c00a859460298f6a2a05dd024fd5a007a296765bc9e336e88dead5198745e951322138756ab213d83fc",
        "from": "Mx1111111111111111111111111111111111111111",
        "nonce": "4",
        "gas_price": 4,
        "type": 4,
        "data": {
          "coin_to_buy": "BIP",
          "value_to_buy": "20000000000000000000000",
          "coin_to_sell": "MNT",
          "maximum_value_to_sell": "9007199254740991000000000000000000"
        },
        "payload": "",
        "service_data": "",
        "gas": "4",
        "gas_coin": "BIP",
        "tags": {
          "tx.return": "23031238072447049784",
          "tx.type": "04",
          "tx.from": "1111111111111111111111111111111111111111",
          "tx.coin_to_buy": "BIP",
          "tx.coin_to_sell": "MNT"
        }
      },
      {
        "hash": "Mt238fdf48e703cdd68203f61eff4b6dbdc45aee27d69ff019ed80e8e92c722051",
        "raw_tx": "f8c10101018a4249500000000000000005b866f864b25468652073746162696c697479206f662074686520646f6c6c617220617420746865207370656564206f662063727970746f8a555344424950000000008c033b2e3c9fd0803ce80000008a021e19e0c9bab2400000648c033b2e3c9fd0803ce8000000808001b845f8431ba06d2f693d54c2b2fc9e9e075997202621517269a8283d1e850d6148c446b3aa1ea0166ee606d46052c9d5ca52a2d074db383d4afe53e76b4775c67a8e9a36fa7cd6",
        "from": "Mx1111111111111111111111111111111111111111",
        "nonce": "5",
        "gas_price": 5,
        "type": 5,
        "data": {
          "name": "Test",
          "symbol": "MNT",
          "initial_amount": "1000000000000000000000000000",
          "initial_reserve": "10000000000000000000000",
          "constant_reserve_ratio": "100",
          "max_supply": "1000000000000000000000000000"
        },
        "payload": "",
        "service_data": "",
        "gas": "5",
        "gas_coin": "BIP",
        "tags": {
          "tx.from": "1111111111111111111111111111111111111111",
          "tx.coin": "MNT",
          "tx.type": "05"
        }
      },
      {
        "hash": "Mt379f8ac7000605485340e0a578d94b37e67beca3e66b41f11bd8f4f35aacda08",
        "raw_tx": "f8a84401018a4249500000000000000006b84df84b942dc21f4ac4a4494cb8b0f768ad065bf505435cafa00d29a83e54653a1d5f34e561e0135f1e81cbcae152f1f327ab36857a7e32de12018a42495000000000000000880de0b6b3a7640000808001b845f8431ba04e9efd36d439f32ce5ae391dbb5166e501087dc6986f794496a0e2cebd23637da022ee419ce856aaab3ca23fa52dcd05db6329e6cf3c186d39987df3d24a9a9150",
        "from": "Mx1111111111111111111111111111111111111111",
        "nonce": "6",
        "gas_price": 6,
        "type": 6,
        "data": {
          "address": "Mx1111111111111111111111111111111111111111",
          "pub_key": "Mpfe176f944623a8ca9f409a62f0ea3ca75c1cf8d89970adf9384fc9ae8d77fa0b",
          "commission": "1",
          "coin": "BIP",
          "stake": "1000000000000000000"
        },
        "payload": "",
        "service_data": "",
        "gas": "6",
        "gas_coin": "BIP",
        "tags": {
          "tx.from": "1111111111111111111111111111111111111111",
          "tx.type": "06"
        }
      },
      {
        "hash": "Mtd8139165198a8afed57aba1824745a2337a24172772bd8c8fa3f4188b865a759",
        "raw_tx": "f892822f4801018a4249500000000000000007b7f6a0fe176f944623a8ca9f409a62f0ea3ca75c1cf8d89970adf9384fc9ae8d77fa0b8a4249500000000000000089014bdbac1b1a76d798808001b844f8421ca033dfe8d5e0eb9c98439a2eb1f1dceb3448ed202c4a5dcd68876333796a30ac909f65511abbd775d7d423ae81af51d6daac8522ab08cc52106e55180c20a64da2",
        "from": "Mx1111111111111111111111111111111111111111",
        "nonce": "7",
        "gas_price": 7,
        "type": 7,
        "data": {
          "pub_key": "Mpfe176f944623a8ca9f409a62f0ea3ca75c1cf8d89970adf9384fc9ae8d77fa0b",
          "coin": "BIP",
          "value": "23912895878861871000"
        },
        "payload": "",
        "service_data": "",
        "gas": "7",
        "gas_coin": "BIP",
        "tags": {
          "tx.type": "07",
          "tx.from": "1111111111111111111111111111111111111111"
        }
      },
      {
        "hash": "Mtab43730e4ff7b00ccae7a060494da44abb4589ff5af4c8e7f1e2dd6140a4a6d8",
        "raw_tx": "f89481ed01018a4249500000000000000008b838f7a04881ad167ca5fb5886322841f992d68aed894ffcb58abc080e8ad3b156f1045b8a465546454c4c313400008a010f0cf064dd59200000808001b845f8431ba0a91a9bcd5ea45e71ca972b37f2f1f20af0242f1e2279cd346d10292b9bd72ed8a00832ad48a677affa031635c589543df1fda0b350cc2cba4122e06f2735177e26",
        "from": "Mx1111111111111111111111111111111111111111",
        "nonce": "8",
        "gas_price": 8,
        "type": 8,
        "data": {
          "pub_key": "Mpfe176f944623a8ca9f409a62f0ea3ca75c1cf8d89970adf9384fc9ae8d77fa0b",
          "coin": "MNT",
          "value": "5000000000000000000000"
        },
        "payload": "",
        "service_data": "",
        "gas": "8",
        "gas_coin": "BIP",
        "tags": {
          "tx.type": "08",
          "tx.from": "1111111111111111111111111111111111111111"
        }
      },
      {
        "hash": "Mt228229b7fd2c5c1e1d84ccecc32a5d2b05612922465c6f22ec6a35cc59024fa1",
        "raw_tx": "f9015a2c01018a4249500000000000000009b8fff8fdb8b8f8b68a3130313234313238323201843b9ac9ff8a42495000000000000000888a5c8e2d3aa500008a42495000000000000000b8415c926bfe64b63c4046f86abb45be7483198ac35e2f6c2b35b78446346bf9bf1f6cad75518c1b8690f517598128f17155b6a0d53f0cd50aa68ba29bf7a39ad9e4011ba0c843adc1073eef3642923ccecc577038ff5ff490633565c41ba2510caa7a5f52a074d0378d8f2e17a5e637df00343f5ea9343b67334dfb69e482441ee61c7bc50eb8417e0c601824952179231f0c262d3da0af1c60bd3e453527edc3a0fa226798fbd66b82fad62326bfe50687af7d497aefff5949d7f7dd3b3f9e77de455d6c2c8db701808001b845f8431ca01f9a862fd39662b1f1d24b73826e0b929fd5fc7a8298e905a92a3324f18f772fa026245877ed939bc28aa1da917f58bebaf938a77331b8b3d157a9ecf69dd30bb1",
        "from": "Mx1111111111111111111111111111111111111111",
        "nonce": "9",
        "gas_price": 9,
        "type": 9,
        "data": {
          "raw_check": "+LaKMTAxMjQxMjgyMgGEO5rJ/4pCSVAAAAAAAAAAiIpcji06pQAAikJJUAAAAAAAAAC4QVySa/5ktjxARvhqu0W+dIMZisNeL2wrNbeERjRr+b8fbK11UYwbhpD1F1mBKPFxVbag1T8M1Qqmi6Kb96Oa2eQBG6DIQ63BBz7vNkKSPM7MV3A4/1/0kGM1ZcQbolEMqnpfUqB00DeNjy4XpeY33wA0P16pNDtnM037aeSCRB7mHHvFDg==",
          "proof": "fgxgGCSVIXkjHwwmLT2grxxgvT5FNSftw6D6ImeY+9ZrgvrWIya/5QaHr31Jeu//WUnX9907P5533kVdbCyNtwE="
        },
        "payload": "",
        "service_data": "",
        "gas": "9",
        "gas_coin": "BIP",
        "tags": {
          "tx.from": "1111111111111111111111111111111111111111",
          "tx.to": "2222222222222222222222222222222222222222",
          "tx.coin": "BIP",
          "tx.type": "09"
        }
      },
      {
        "hash": "Mte2498851fc38a775d1e90fdf745b81a26926afdd2ebd3ae5f80f9ca75704d1eb",
        "raw_tx": "f87d819501018a424950000000000000000aa2e1a001cc99ae5a349ecaeef187dcbb12816bf2b3d8eae80f654034b21213aa445b2c808001b845f8431ba065e19cd7c78ed0d06abeff25ef3a375fa6141b5096121bbc5097ba6948d12590a04c9124e54de6bbb82cec0e3040e312b2fdd92d5116b0e0878c35331dfc2cf1dc",
        "from": "Mx1111111111111111111111111111111111111111",
        "nonce": "10",
        "gas_price": 10,
        "type": 10,
        "data": {
          "pub_key": "Mpfe176f944623a8ca9f409a62f0ea3ca75c1cf8d89970adf9384fc9ae8d77fa0b"
        },
        "payload": "",
        "service_data": "",
        "gas": "10",
        "gas_coin": "BIP",
        "tags": {
          "tx.type": "0a",
          "tx.from": "1111111111111111111111111111111111111111"
        }
      },
      {
        "hash": "Mt62aac4a96e437fcf6a06ea71831141aff4c792e486d7b292e94c4d346f3a5090",
        "raw_tx": "f87c0101018a424950000000000000000ba2e1a07b4174732a169c467c9ee791576fc5860ca99bc7a49d8cfb041a91f9202178cc808001b845f8431ca04343d46dc8652b7b00bc15a8f871276112a30b7b708cff0d11e2ad67800346c3a03d0322fbd2e21fc1ccf54f3daa22dd033b73dc595f27ae0b5936f10a5cca8bf5",
        "from": "Mx1111111111111111111111111111111111111111",
        "nonce": "11",
        "gas_price": 11,
        "type": 11,
        "data": {
          "pub_key": "Mpfe176f944623a8ca9f409a62f0ea3ca75c1cf8d89970adf9384fc9ae8d77fa0b"
        },
        "payload": "",
        "service_data": "",
        "gas": "11",
        "gas_coin": "BIP",
        "tags": {
          "tx.type": "0b",
          "tx.from": "1111111111111111111111111111111111111111"
        }
      },
		{
		  "hash": "Mt8cf0bf9f93bbdae30b8641ef0f62709ef13bc3dda27326f353365553498e7da4",
		  "raw_tx": "f88a0101018a424950000000000000000cb0ef0ac20505ea9462b0f42435a64822ff133149b4a12c9e0f756eea945ba0e3efa1ac05a9bbbbb59000b800169040b2c7808001b845f8431ba0402984628e67f6e45249e31b4dbaf53e27863009402128319ccaf94843bde404a068cd3fe6289d381de291055b57cec5811ba9070f410e4e1420aa2a1a3df1fabe",
		  "from": "Mx1111111111111111111111111111111111111111",
		  "nonce": "12",
		  "gas_price": 12,
		  "type": 12,
		  "data": {
			"threshold": "10",
			"weights": [
			  "5",
			  "5"
			],
			"addresses": [
			  "Mx2222222222222222222222222222222222222222",
			  "Mx3333333333333333333333333333333333333333"
			]
		  },
		  "payload": "",
		  "service_data": "",
		  "gas": "12",
		  "gas_coin": "BIP",
		  "tags": {
			"tx.from": "1111111111111111111111111111111111111111",
			"tx.created_multisig": "c1ed5ec59e112fb485f0658b8a04ea50c1c5e2c6",
			"tx.type": "0c"
		  }
		},
      {
        "hash": "Mt9dd5e2963c9bbc115447bcaabb659906c6d5fef82ae0524e28b1d8030850ebd0",
        "raw_tx": "f88782279701018a574f524c44434f494e000dabeae9e88a574f524c44434f494e00941ee3bcc0ec12149f887f36c8f776a8af7f5e0232874154a96496c000808001b845f8431ba0cb98641b8c7f9e9c43628bd0e7dad093a49680906747613682271806479dd689a079dc75a36953722c9e3bead0eda20bcd8d82a819c6432ca50107221ed8478fae",
        "from": "Mx1111111111111111111111111111111111111111",
        "nonce": "13",
        "gas_price": 13,
        "type": 13,
        "data": {
          "list": [
            {
              "coin": "BIP",
              "to": "Mx2222222222222222222222222222222222222222",
              "value": "18388960000000000"
            }
          ]
        },
        "payload": "",
        "service_data": "",
        "gas": "13",
        "gas_coin": "BIP",
        "tags": {
          "tx.type": "0d",
          "tx.from": "1111111111111111111111111111111111111111",
          "tx.to": "2222222222222222222222222222222222222222"
        }
      },
      {
        "hash": "Mt623775719be2db286626b9fe4e9d31152039151779252616f76c56a159966a3d",
        "raw_tx": "f8aa82053701018a424950000000000000000eb84df84ba0eee9614b63a7ed6370ccd1fa227222fa30d6106770145c55bd4b482b88888888947a77cd2baf195c2e33194ef9f2ca7295452e1777943cfad1cf0f8b097b52f8b0d7fcdbba4123914a6c808001b845f8431ba0262b3b4b092597e169c8cdc8780a8aba42fe7a9d3aa0ec6bc3cca0ba906e2c8da0060d67f21dd7a5504fa47fe777c6def171235234bafbb1d68894b820a2955158",
        "from": "Mx1111111111111111111111111111111111111111",
        "nonce": "14",
        "gas_price": 14,
        "type": 14,
        "data": {
          "pub_key": "Mpfe176f944623a8ca9f409a62f0ea3ca75c1cf8d89970adf9384fc9ae8d77fa0b",
          "reward_address": "Mx1111111111111111111111111111111111111111",
          "owner_address": "Mx1111111111111111111111111111111111111111"
        },
        "payload": "",
        "service_data": "",
        "gas": "14",
        "gas_coin": "BIP",
        "tags": {
          "tx.type": "0e",
          "tx.from": "1111111111111111111111111111111111111111"
        }
      }
    ],
    "block_reward": "308000000000000000000",
    "size": "4542",
    "proposer": "Mpfe176f944623a8ca9f409a62f0ea3ca75c1cf8d89970adf9384fc9ae8d77fa0b",
    "validators": [
      {
        "pub_key": "Mpfe176f944623a8ca9f409a62f0ea3ca75c1cf8d89970adf9384fc9ae8d77fa0b",
        "signed": true
      }
    ]
  }
}
    `
	c.String(http.StatusOK, resp)
}

func fakeStatus(c *gin.Context) {
	s := `{"jsonrpc":"2.0","id":"","result":{"version":"1.1.3","latest_block_hash":"957805E05000ED34CBB8056ECC3C215A51768EA505FBFAC7DC8E9A5CE42DA5D9","latest_app_hash":"7C67FC9E966C93B6D34D41BF9A21DA1DBD6D2C7E81A801CD84B152F385E865EB","latest_block_height":"15","latest_block_time":"2020-04-01T07:49:49.4575554Z","keep_last_states":"100000","tm_status":{"node_info":{"protocol_version":{"p2p":"7","block":"10","app":"6"},"id":"f731724174177dc6c2c733e08fbb7f7098046560","listen_addr":"tcp://0.0.0.0:26656","network":"minter-mainnet-2","version":"0.33.2","channels":"4020212223303800","moniker":"explorer-node.minter.network","other":{"tx_index":"on","rpc_address":"tcp://127.0.0.1:26657"}},"sync_info":{"latest_block_hash":"957805E05000ED34CBB8056ECC3C215A51768EA505FBFAC7DC8E9A5CE42DA5D9","latest_app_hash":"7C67FC9E966C93B6D34D41BF9A21DA1DBD6D2C7E81A801CD84B152F385E865EB","latest_block_height":"474521","latest_block_time":"2020-04-01T07:49:49.4575554Z","catching_up":false},"validator_info":{"address":"0CDCAEB7E32CFE9BB3F796BBF12F3F1A9864753D","pub_key":{"type":"tendermint/PubKeyEd25519","value":"BwKV9RRxTEhz1vNuUTzbJbIJXZissTp29M9C8Y/6zmM="},"voting_power":"0"}}}}`
	c.String(http.StatusOK, s)
}

func fakeEvents(c *gin.Context) {
	response := `
	{
	  "jsonrpc": "2.0",
	  "id": "",
	  "result": {
		"events": [
			{
				"type":"minter/RewardEvent",
				"value":{
					"role":"DAO",
					"address":"Mx1111111111111111111111111111111111111111",
					"amount":"10000000000000000",
					"validator_pub_key":"Mpfe176f944623a8ca9f409a62f0ea3ca75c1cf8d89970adf9384fc9ae8d77fa0b"
				}
			},
			{
				"type": "minter/SlashEvent",
				"value": {
					"address": "Mx1111111111111111111111111111111111111111",
					"amount": "10000000000000000",
					"coin": "BIP",
					"validator_pub_key": "Mpfe176f944623a8ca9f409a62f0ea3ca75c1cf8d89970adf9384fc9ae8d77fa0b"
				}
			}
	  	]}
	}
	`
	c.String(http.StatusOK, response)
}

func fakeAddresses(c *gin.Context) {
	response := `{"jsonrpc":"2.0","id":"","result":[`
	response += `{"address":"Mx1111111111111111111111111111111111111111","balance":{"MNT":"10000000000000000", "BIP":"43535144074175544767"},"transaction_count":"10134"},`
	response += `{"address":"Mx2222222222222222222222222222222222222222","balance":{"MNT":"10000000000000000", "BIP":"43535144074175544767"},"transaction_count":"30"},`
	response += `{"address":"Mxdf1050b032d6feb23f00e65522820fa94838401b","balance":{"MNT":"10000000000000000", "BIP":"43535144074175544767"},"transaction_count":"30"}`
	response += `]}`
	c.String(http.StatusOK, response)
}

func fakeCoinInfo(c *gin.Context) {
	coin := c.Query("symbol")
	s := `{"jsonrpc":"2.0","id":"","result":{"name":"Test coin","symbol":"` + coin + `","volume":"10000000000000000000000","crr":"10","reserve_balance":"5000000000000000000000","max_supply":"1000000000000000000000000000000000"}}`
	c.String(http.StatusOK, s)
}

func fakeCandidates(c *gin.Context) {
	includeStakes := c.Query("include_stakes")

	stakes := ""
	if includeStakes == "true" {
		stakes = `"stakes":[{"owner":"Mx1111111111111111111111111111111111111111","coin":"BIP","value":"1111111111111111111111111111111111111111","bip_value":"1111111111111111111111111111111111111111"}],`
	}
	response := `{"jsonrpc":"2.0","id":"","result":[{`
	response += `"reward_address": "Mx1111111111111111111111111111111111111111",`
	response += `"owner_address": "Mx1111111111111111111111111111111111111111",`
	response += `"total_stake": "1111111111111111111111111111111111111111",`
	response += `"pub_key": "Mpfe176f944623a8ca9f409a62f0ea3ca75c1cf8d89970adf9384fc9ae8d77fa0b",`
	response += `"commission": "5",`
	response += stakes
	response += `"status": 2`
	response += `}]}`

	c.String(http.StatusOK, response)
}
