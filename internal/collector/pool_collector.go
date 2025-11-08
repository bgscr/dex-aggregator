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

// helper function to create big.Int from string
func bigIntFromString(s string) *big.Int {
	result, ok := new(big.Int).SetString(s, 10)
	if !ok {
		log.Printf("Warning: Failed to parse big int from string: %s", s)
		return big.NewInt(0)
	}
	return result
}

func (mpc *MockPoolCollector) InitMockPools() error {
	ctx := context.Background()

	// 扩展代币列表
	tokens := map[string]types.Token{
		"WETH": {
			Address:  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
			Symbol:   "WETH",
			Decimals: 18,
		},
		"USDT": {
			Address:  "0xdac17f958d2ee523a2206206994597c13d831ec7",
			Symbol:   "USDT",
			Decimals: 6,
		},
		"USDC": {
			Address:  "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
			Symbol:   "USDC",
			Decimals: 6,
		},
		"DAI": {
			Address:  "0x6b175474e89094c44da98b954eedeac495271d0f",
			Symbol:   "DAI",
			Decimals: 18,
		},
		"WBTC": {
			Address:  "0x2260fac5e5542a773aa44fbcfedf7c193bc2c599",
			Symbol:   "WBTC",
			Decimals: 8,
		},
		"LINK": {
			Address:  "0x514910771af9ca656af840dff83e8264ecf986ca",
			Symbol:   "LINK",
			Decimals: 18,
		},
		"UNI": {
			Address:  "0x1f9840a85d5af5bf1d1762f925bdaddc4201f984",
			Symbol:   "UNI",
			Decimals: 18,
		},
		"AAVE": {
			Address:  "0x7fc66500c84a76ad7e9c93437bfc5ac33e2ddae9",
			Symbol:   "AAVE",
			Decimals: 18,
		},
	}

	// 扩展交易对
	pairs := []struct {
		name     string
		token0   types.Token
		token1   types.Token
		reserve0 *big.Int
		reserve1 *big.Int
	}{
		// 主要稳定币对
		{
			name:     "WETH/USDT",
			token0:   tokens["WETH"],
			token1:   tokens["USDT"],
			reserve0: bigIntFromString("10000000000000000000"), // 10 WETH
			reserve1: big.NewInt(20000000000),                  // 20,000 USDT
		},
		{
			name:     "WETH/USDC",
			token0:   tokens["WETH"],
			token1:   tokens["USDC"],
			reserve0: bigIntFromString("500000000000000000000"), // 500 WETH
			reserve1: big.NewInt(100000000000),                  // 1,000,000 USDC
		},
		{
			name:     "WETH/DAI",
			token0:   tokens["WETH"],
			token1:   tokens["DAI"],
			reserve0: bigIntFromString("3000000000000000000"),    // 3 WETH
			reserve1: bigIntFromString("6000000000000000000000"), // 6000 DAI
		},
		// 稳定币间交易对
		{
			name:     "USDC/USDT",
			token0:   tokens["USDC"],
			token1:   tokens["USDT"],
			reserve0: big.NewInt(5000000000), // 5,000 USDC
			reserve1: big.NewInt(5000000000), // 5,000 USDT
		},
		{
			name:     "USDC/DAI",
			token0:   tokens["USDC"],
			token1:   tokens["DAI"],
			reserve0: big.NewInt(3000000000),                     // 3,000 USDC
			reserve1: bigIntFromString("3000000000000000000000"), // 3000 DAI
		},
		// WBTC 交易对
		{
			name:     "WETH/WBTC",
			token0:   tokens["WETH"],
			token1:   tokens["WBTC"],
			reserve0: bigIntFromString("50000000000000000000"), // 50 WETH
			reserve1: big.NewInt(200000000),                    // 2 WBTC
		},
		{
			name:     "WBTC/USDT",
			token0:   tokens["WBTC"],
			token1:   tokens["USDT"],
			reserve0: big.NewInt(100000000),   // 1 WBTC
			reserve1: big.NewInt(30000000000), // 30,000 USDT
		},
		// 其他代币对
		{
			name:     "WETH/LINK",
			token0:   tokens["WETH"],
			token1:   tokens["LINK"],
			reserve0: bigIntFromString("2000000000000000000"),    // 2 WETH
			reserve1: bigIntFromString("2000000000000000000000"), // 2000 LINK
		},
		{
			name:     "WETH/UNI",
			token0:   tokens["WETH"],
			token1:   tokens["UNI"],
			reserve0: bigIntFromString("1000000000000000000"),   // 1 WETH
			reserve1: bigIntFromString("500000000000000000000"), // 500 UNI
		},
		{
			name:     "WETH/AAVE",
			token0:   tokens["WETH"],
			token1:   tokens["AAVE"],
			reserve0: bigIntFromString("800000000000000000"),   // 0.8 WETH
			reserve1: bigIntFromString("40000000000000000000"), // 40 AAVE
		},
		// 三跳路径需要的交易对
		{
			name:     "LINK/USDT",
			token0:   tokens["LINK"],
			token1:   tokens["USDT"],
			reserve0: bigIntFromString("5000000000000000000000"), // 5000 LINK
			reserve1: big.NewInt(2500000000),                     // 2,500 USDT
		},
		{
			name:     "UNI/USDC",
			token0:   tokens["UNI"],
			token1:   tokens["USDC"],
			reserve0: bigIntFromString("1000000000000000000000"), // 1000 UNI
			reserve1: big.NewInt(2000000000),                     // 2,000 USDC
		},
		// 在 pairs 数组中增加更高流动性的池子
		{
			name:     "WETH/USDT-HighLiquidity",
			token0:   tokens["WETH"],
			token1:   tokens["USDT"],
			reserve0: bigIntFromString("100000000000000000000"), // 100 WETH
			reserve1: big.NewInt(200000000000),                  // 200,000 USDT
		},
	}

	// 为每个交易所创建池子
	uniquePools := make(map[string]bool)
	poolCount := 0
	for _, exchange := range mpc.exchanges {
		for i, pair := range pairs {

			poolAddress := fmt.Sprintf("%s-%s-%d",
				strings.ToLower(strings.ReplaceAll(exchange.Name, " ", "")),
				strings.ToLower(strings.ReplaceAll(pair.name, "/", "-")),
				i)
			if uniquePools[poolAddress] {
				continue
			}
			uniquePools[poolAddress] = true

			pool := &types.Pool{
				Address:     fmt.Sprintf("%s-%s-%d", strings.ToLower(strings.ReplaceAll(exchange.Name, " ", "")), strings.ToLower(strings.ReplaceAll(pair.name, "/", "-")), i),
				Exchange:    exchange.Name,
				Version:     exchange.Version,
				Token0:      pair.token0,
				Token1:      pair.token1,
				Reserve0:    pair.reserve0,
				Reserve1:    pair.reserve1,
				Fee:         300,
				LastUpdated: time.Now(),
			}

			err := mpc.cache.StorePool(ctx, pool)
			if err != nil {
				log.Printf("Failed to store pool: %v", err)
			} else {
				log.Printf("✓ Created %s pool: %s", exchange.Name, pair.name)
				poolCount++
			}
		}
	}

	log.Printf("Successfully created %d mock pools across %d exchanges", poolCount, len(mpc.exchanges))
	return nil
}
