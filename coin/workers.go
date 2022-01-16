package coin

import (
	"github.com/MinterTeam/minter-explorer-extender/v2/models"
	"github.com/MinterTeam/minter-go-sdk/v2/transaction"
	"github.com/MinterTeam/node-grpc-gateway/api_pb"
	"google.golang.org/protobuf/types/known/anypb"
	"strconv"
	"strings"
	"time"
)

func (s *Service) UpdateHubInfoWorker() {
	for {
		//Check if testnet
		if s.env.BaseCoin == "MNT" {
			continue
		}

		hubResponse := new(models.HubCoinsInfoResponse)
		resp, err := s.httpClient.R().
			SetResult(&hubResponse).
			Get("https://hub-api.minter.network/mhub2/v1/token_infos")

		if err != nil {
			s.logger.Error(err)
			continue
		}

		if resp.IsError() {
			s.logger.Error("bad response")
			continue
		}

		tokensMap := make(map[string]map[string]models.TokenInfo)
		for _, ci := range hubResponse.List.Tokens {
			if tokensMap[ci.Denom] == nil {
				tokensMap[ci.Denom] = make(map[string]models.TokenInfo)
			}
			tokensMap[ci.Denom][ci.ChainId] = ci
		}

		var list []models.TokenContract
		var ids []uint64
		for _, t := range tokensMap {
			coinId, err := strconv.ParseUint(t["minter"].ExternalTokenId, 10, 64)
			if err != nil {
				s.logger.Error(err)
				continue
			}
			ids = append(ids, coinId)
			list = append(list, models.TokenContract{
				CoinId: coinId,
				Eth:    strings.ToLower(t["ethereum"].ExternalTokenId),
				Bsc:    strings.ToLower(t["bsc"].ExternalTokenId),
			})
		}

		err = s.Storage.SaveTokenContracts(list)
		if err != nil {
			s.logger.Error(err)
			continue
		}

		time.Sleep(5 * time.Minute)
	}
}

func (s *Service) UpdateCoinsInfoFromTxsWorker(jobs <-chan []*models.Transaction) {
	for transactions := range jobs {
		coinsMap := make(map[uint64]struct{})
		// Find coins in transaction for update
		for _, tx := range transactions {

			coinsMap[tx.GasCoinID] = struct{}{}

			switch transaction.Type(tx.Type) {
			case transaction.TypeSellCoin:
				txData := new(api_pb.SellCoinData)
				if err := tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
					s.logger.Error(err)
					continue
				}
				coinsMap[txData.CoinToBuy.Id] = struct{}{}
				coinsMap[txData.CoinToSell.Id] = struct{}{}
			case transaction.TypeBuyCoin:
				txData := new(api_pb.BuyCoinData)
				if err := tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
					s.logger.Error(err)
					continue
				}
				coinsMap[txData.CoinToBuy.Id] = struct{}{}
				coinsMap[txData.CoinToSell.Id] = struct{}{}
			case transaction.TypeSellAllCoin:
				txData := new(api_pb.SellAllCoinData)
				if err := tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
					s.logger.Error(err)
					continue
				}
				coinsMap[txData.CoinToBuy.Id] = struct{}{}
				coinsMap[txData.CoinToSell.Id] = struct{}{}
			case transaction.TypeBuySwapPool:
				txData := new(api_pb.BuySwapPoolData)
				if err := tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
					s.logger.Error(err)
					continue
				}
				for _, c := range txData.Coins {
					coinsMap[c.Id] = struct{}{}
				}
			case transaction.TypeSellSwapPool:
				txData := new(api_pb.SellSwapPoolData)
				if err := tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
					s.logger.Error(err)
					continue
				}
				for _, c := range txData.Coins {
					coinsMap[c.Id] = struct{}{}
				}
			case transaction.TypeSellAllSwapPool:
				txData := new(api_pb.SellAllSwapPoolData)
				if err := tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
					s.logger.Error(err)
					continue
				}
				for _, c := range txData.Coins {
					coinsMap[c.Id] = struct{}{}
				}
			case transaction.TypeMintToken:
				txData := new(api_pb.MintTokenData)
				if err := tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
					s.logger.Error(err)
					continue
				}
				coinsMap[txData.Coin.Id] = struct{}{}
			case transaction.TypeBurnToken:
				txData := new(api_pb.BurnTokenData)
				if err := tx.IData.(*anypb.Any).UnmarshalTo(txData); err != nil {
					s.logger.Error(err)
					continue
				}
				coinsMap[txData.Coin.Id] = struct{}{}
			}
		}
		s.GetUpdateCoinsFromCoinsMapJobChannel() <- coinsMap
	}
}
