package main

import (
	"dex-aggregator/internal/types"
	"math/big"
	"math/rand"
	"time"
)

// TestToken 创建测试代币
func TestToken(address, symbol string, decimals int) types.Token {
	return types.Token{
		Address:  address,
		Symbol:   symbol,
		Decimals: decimals,
	}
}

// TestPool 创建测试池子
func TestPool(address, exchange string, token0, token1 types.Token, reserve0, reserve1 *big.Int) *types.Pool {
	return &types.Pool{
		Address:     address,
		Exchange:    exchange,
		Version:     "v2",
		Token0:      token0,
		Token1:      token1,
		Reserve0:    reserve0,
		Reserve1:    reserve1,
		Fee:         300,
		LastUpdated: time.Now(),
	}
}

// RandomBigInt 生成随机的大整数
func RandomBigInt(min, max int64) *big.Int {
	rand.Seed(time.Now().UnixNano())
	value := rand.Int63n(max-min) + min
	return big.NewInt(value)
}

// CreateMockPools 创建模拟池子数据用于测试
func CreateMockPools() []*types.Pool {
	weth := TestToken("0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2", "WETH", 18)
	usdt := TestToken("0xdac17f958d2ee523a2206206994597c13d831ec7", "USDT", 6)
	usdc := TestToken("0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48", "USDC", 6)
	dai := TestToken("0x6b175474e89094c44da98b954eedeac495271d0f", "DAI", 18)

	// 使用 big.Int 来处理大数
	daiReserve, _ := new(big.Int).SetString("1600000000000000000000", 10)

	return []*types.Pool{
		TestPool("pool1", "Uniswap V2", weth, usdt, big.NewInt(1000000000000000000), big.NewInt(2000000000)),
		TestPool("pool2", "Uniswap V2", weth, usdc, big.NewInt(500000000000000000), big.NewInt(1000000000)),
		TestPool("pool3", "SushiSwap", weth, dai, big.NewInt(800000000000000000), daiReserve),
		TestPool("pool4", "Uniswap V2", usdc, usdt, big.NewInt(500000000), big.NewInt(500000000)),
	}
}
