package main

import (
	"context"
	"dex-aggregator/config"
	"dex-aggregator/internal/aggregator"
	"dex-aggregator/internal/cache"
	"dex-aggregator/internal/types"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestIntegration_CompleteFlow tests complete quote flow
func TestIntegration_CompleteFlow(t *testing.T) {
	// Initialize configuration
	err := config.Init()
	assert.NoError(t, err)

	// Create in-memory cache for testing
	store := cache.NewMemoryStore()

	// Create test pool - using big.Int SetString for large numbers
	reserve0, _ := new(big.Int).SetString("10000000000000000000", 10) // 10 ETH
	reserve1 := big.NewInt(20000000000)                               // 20000 USDT

	pool := &types.Pool{
		Address:  "test-integration-pool",
		Exchange: "Uniswap V2",
		Token0: types.Token{
			Address:  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
			Symbol:   "WETH",
			Decimals: 18,
		},
		Token1: types.Token{
			Address:  "0xdac17f958d2ee523a2206206994597c13d831ec7",
			Symbol:   "USDT",
			Decimals: 6,
		},
		Reserve0: reserve0,
		Reserve1: reserve1,
		Fee:      300,
	}

	// Store pool *BEFORE* initializing the router
	// This ensures the PathFinder's initial graph load sees this pool.
	err = store.StorePool(context.Background(), pool)
	assert.NoError(t, err)

	// Create router
	perfConfig := config.PerformanceConfig{MaxSlippage: 5.0, MaxHops: 3, MaxConcurrentPaths: 10, CacheTTL: 60 * time.Second}
	router := aggregator.NewRouter(store, perfConfig)
	assert.NotNil(t, router)

	// Create quote request - using smaller amount
	req := &types.QuoteRequest{
		TokenIn:  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
		TokenOut: "0xdac17f958d2ee523a2206206994597c13d831ec7",
		AmountIn: big.NewInt(1000000000000000), // 0.001 ETH - smaller amount
		MaxHops:  3,
	}

	// Get quote
	response, err := router.GetBestQuote(context.Background(), req)

	// In this test, we now EXPECT to find a path.
	assert.NoError(t, err)
	assert.NotNil(t, response)
	if response != nil {
		assert.NotNil(t, response.AmountOut)
		// Check that amount out is positive
		assert.True(t, response.AmountOut.Cmp(big.NewInt(0)) > 0)
	}
}

// TestIntegration_CachePerformance tests cache performance
func TestIntegration_CachePerformance(t *testing.T) {
	store := cache.NewMemoryStore() // Use in-memory cache instead of Redis

	// Multiple accesses to test cache hit rate
	pool := &types.Pool{
		Address:  "performance-test-pool",
		Exchange: "Uniswap V2",
		Token0:   types.Token{Address: "0xtokena"},
		Token1:   types.Token{Address: "0xtokenb"},
	}

	err := store.StorePool(context.Background(), pool)
	assert.NoError(t, err)

	// First access
	retrievedPool, err := store.GetPool(context.Background(), "performance-test-pool")
	assert.NoError(t, err)
	assert.Equal(t, pool.Address, retrievedPool.Address)

	// Second access - should hit cache
	retrievedPool2, err := store.GetPool(context.Background(), "performance-test-pool")
	assert.NoError(t, err)
	assert.Equal(t, pool.Address, retrievedPool2.Address)
}
