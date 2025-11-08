package collector

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"dex-aggregator/internal/cache"
	"dex-aggregator/internal/types"
)

type MockPoolCollector struct {
	cache     cache.Store
	exchanges []*types.Exchange
}

func NewMockPoolCollector(cache cache.Store) *MockPoolCollector {
	exchanges := []*types.Exchange{
		{
			Name:    "Uniswap V2",
			Factory: "0x5C69bEe701ef814a2B6a3EDD4B1652CB9cc5aA6f",
			Router:  "0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D",
			Version: "v2",
		},
		{
			Name:    "SushiSwap",
			Factory: "0xC0AEe478e3658e2610c5F7A4A2E1777cE9e4f2Ac",
			Router:  "0xd9e1cE17f2641f24aE83637ab66a2cca9C378B9F",
			Version: "v2",
		},
	}

	return &MockPoolCollector{
		cache:     cache,
		exchanges: exchanges,
	}
}

func (mpc *MockPoolCollector) InitMockPools() error {
	ctx := context.Background()

	majorPairs := []struct {
		name   string
		token0 types.Token
		token1 types.Token
	}{
		{
			name: "WETH/USDT",
			token0: types.Token{
				Address:  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
				Symbol:   "WETH",
				Decimals: 18,
			},
			token1: types.Token{
				Address:  "0xdac17f958d2ee523a2206206994597c13d831ec7",
				Symbol:   "USDT",
				Decimals: 6,
			},
		},
		{
			name: "WETH/USDC",
			token0: types.Token{
				Address:  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
				Symbol:   "WETH",
				Decimals: 18,
			},
			token1: types.Token{
				Address:  "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
				Symbol:   "USDC",
				Decimals: 6,
			},
		},
		{
			name: "WETH/DAI",
			token0: types.Token{
				Address:  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
				Symbol:   "WETH",
				Decimals: 18,
			},
			token1: types.Token{
				Address:  "0x6b175474e89094c44da98b954eedeac495271d0f",
				Symbol:   "DAI",
				Decimals: 18,
			},
		},
		{
			name: "USDC/USDT",
			token0: types.Token{
				Address:  "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
				Symbol:   "USDC",
				Decimals: 6,
			},
			token1: types.Token{
				Address:  "0xdac17f958d2ee523a2206206994597c13d831ec7",
				Symbol:   "USDT",
				Decimals: 6,
			},
		},
	}

	poolCount := 0
	for _, exchange := range mpc.exchanges {
		for i, pair := range majorPairs {
			pool := &types.Pool{
				Address:     fmt.Sprintf("%s-pool-%s-%d", strings.ToLower(strings.ReplaceAll(exchange.Name, " ", "")), pair.name, i),
				Exchange:    exchange.Name,
				Version:     exchange.Version,
				Token0:      pair.token0,
				Token1:      pair.token1,
				Reserve0:    big.NewInt(1000000000000000000), // 1 ETH
				Reserve1:    big.NewInt(2000000000),          // 2000 USDT/USDC
				Fee:         300,
				LastUpdated: time.Now(),
			}

			// Adjust reserves based on token pair
			switch pair.name {
			case "WETH/USDT", "WETH/USDC":
				// These are fine with default values
				pool.Reserve1 = big.NewInt(2000000000)
			case "WETH/DAI":
				// Fix: 2000 DAI = 2000 * 10^18
				pool.Reserve1 = new(big.Int).Mul(big.NewInt(2000), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
			case "USDC/USDT":
				pool.Reserve0 = big.NewInt(1000000000) // 1000 USDC
				pool.Reserve1 = big.NewInt(1000000000) // 1000 USDT
			}

			err := mpc.cache.StorePool(ctx, pool)
			if err != nil {
				log.Printf("Failed to store pool: %v", err)
			} else {
				log.Printf("✓ Created %s pool: %s (Reserves: %s/%s)",
					exchange.Name, pair.name,
					pool.Reserve0.String(), pool.Reserve1.String())
				poolCount++
			}
		}
	}

	log.Printf("Successfully created %d mock pools", poolCount)

	// Verify pools were stored correctly
	allPools, err := mpc.cache.GetAllPools(ctx)
	if err != nil {
		log.Printf("Failed to get pools for verification: %v", err)
	} else {
		log.Printf("Verification: %d pools now in cache", len(allPools))

		// Debug: print token addresses to verify they're lowercase
		for _, pool := range allPools {
			log.Printf("Pool tokens: %s (%s) / %s (%s)",
				pool.Token0.Symbol, pool.Token0.Address,
				pool.Token1.Symbol, pool.Token1.Address)
		}
	}

	return nil
}
