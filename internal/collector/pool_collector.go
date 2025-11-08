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

// MockPoolCollector mock data collector
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

// Initialize mock pool data
func (mpc *MockPoolCollector) InitMockPools() error {
	ctx := context.Background()

	// Major trading pairs - use consistent lowercase addresses
	majorPairs := []struct {
		name   string
		token0 types.Token
		token1 types.Token
	}{
		{
			name: "WETH/USDT",
			token0: types.Token{
				Address:  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2", // WETH lowercase
				Symbol:   "WETH",
				Decimals: 18,
			},
			token1: types.Token{
				Address:  "0xdac17f958d2ee523a2206206994597c13d831ec7", // USDT lowercase
				Symbol:   "USDT",
				Decimals: 6,
			},
		},
		{
			name: "WETH/USDC",
			token0: types.Token{
				Address:  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2", // WETH
				Symbol:   "WETH",
				Decimals: 18,
			},
			token1: types.Token{
				Address:  "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48", // USDC
				Symbol:   "USDC",
				Decimals: 6,
			},
		},
		{
			name: "WETH/DAI",
			token0: types.Token{
				Address:  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2", // WETH
				Symbol:   "WETH",
				Decimals: 18,
			},
			token1: types.Token{
				Address:  "0x6b175474e89094c44da98b954eedeac495271d0f", // DAI
				Symbol:   "DAI",
				Decimals: 18,
			},
		},
		{
			name: "USDC/USDT",
			token0: types.Token{
				Address:  "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48", // USDC
				Symbol:   "USDC",
				Decimals: 6,
			},
			token1: types.Token{
				Address:  "0xdac17f958d2ee523a2206206994597c13d831ec7", // USDT
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
				Reserve0:    big.NewInt(1000000000000000000), // 1 ETH or equivalent
				Reserve1:    big.NewInt(2000000000),          // 2000 USDT/USDC or equivalent
				Fee:         300,
				LastUpdated: time.Now(),
			}

			// Adjust reserves based on token pair
			switch pair.name {
			case "WETH/USDT", "WETH/USDC":
				// These are fine with default values
				pool.Reserve1 = big.NewInt(2000000000)
			case "WETH/DAI":
				// 18位小数的池子，2000 * 1e18
				pool.Reserve1 = new(big.Int).SetBytes(big.NewInt(2000).Mul(big.NewInt(2000), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)).Bytes())
			case "USDC/USDT":
				pool.Reserve0 = big.NewInt(1000000000) // 1000 USDC
				pool.Reserve1 = big.NewInt(1000000000) // 1000 USDT
			}

			err := mpc.cache.StorePool(ctx, pool)
			if err != nil {
				log.Printf("Failed to store pool: %v", err)
			} else {
				log.Printf("✓ Created %s pool: %s (Tokens: %s/%s)",
					exchange.Name, pair.name,
					strings.ToLower(pair.token0.Address),
					strings.ToLower(pair.token1.Address))
				poolCount++
			}
		}
	}

	log.Printf("Successfully created %d mock pools", poolCount)

	// Verify pools were stored
	allPools, _ := mpc.cache.GetAllPools(ctx)
	log.Printf("Verification: %d pools now in cache", len(allPools))

	// Debug: print all stored token addresses
	for _, pool := range allPools {
		log.Printf("Stored pool tokens: %s (%s) / %s (%s)",
			pool.Token0.Symbol, pool.Token0.Address,
			pool.Token1.Symbol, pool.Token1.Address)
	}

	return nil
}
