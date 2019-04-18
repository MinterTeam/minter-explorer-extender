<p align="center" background="black"><img src="minter-logo.svg" width="400"></p>

<p align="center" style="text-align: center;">
    <a href="https://github.com/MinterTeam/minter-explorer-extender/blob/master/LICENSE">
        <img src="https://img.shields.io/packagist/l/doctrine/orm.svg" alt="License">
    </a>
    <img alt="undefined" src="https://img.shields.io/github/last-commit/MinterTeam/minter-explorer-extender.svg">
</p>

# Minter Explorer Extender

The official repository of Minter Explorer Extender service.

Extender is a service responsible for seeding the database from the blockchain network. Part of the Minter Explorer service.

_NOTE: This project in active development stage so feel free to send us questions, issues, and wishes_

<p align="center" background="black"><img src="minter-explorer.jpeg" width="400"></p>

## BUILD

- dep ensure

- replace Minter Node in vendor directory ```cd vendor/github.com/MinterTeam && rm -rf minter-go-node && git clone https://github.com/MinterTeam/minter-go-node.git```

- make build

## RUN

If you run Extender for the first time yoг need to run  [Explorer Genesis Uploader](https://github.com/MinterTeam/explorer-genesis-uploader)
to fill data from genesis file (you can use the same config file for both services)

./extender -config=/etc/minter/config.json

### Config file

Support JSON and YAML formats 

Example:

```
{
  "name": "Minter Extender",
  "app": {
    "debug": true,
    "baseCoin": "MNT",
    "txChunkSize": 200,
    "addrChunkSize": 30,
    "eventsChunkSize": 200
  },
  "workers": {
    "saveTxs": 10,
    "saveTxsOutput": 5,
    "saveInvalidTxs": 2,
    "saveRewards": 3,
    "saveSlashes": 3,
    "saveAddresses": 3,
    "saveTxValidator": 2,
    "updateBalance": 2,
    "balancesFromNode": 3
  },
  "database": {
    "host": "localhost",
    "name": "explorer",
    "user": "minter",
    "password": "password",
    "minIdleConns": 10,
    "poolSize": 20
  },
  "minterApi": {
    "isSecure": false,
    "link": "localhost",
    "port": 8841
  },
  "extenderApi": {
    "host": "",
    "port": 8800
  },
  "wsServer": {
    "isSecure": true,
    "link": "localhost",
    "port": "",
    "key": "secret-key"
  }
}
```
