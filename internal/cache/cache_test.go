package cache

import (
	"context"
	"dex-aggregator/internal/types"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTwoLevelCache_StoreAndGetPool(t *testing.T) {
	// Use real two-level cache, but use memory store to simulate Redis
	// In real environment, this should connect to test Redis instance
	tlc := NewTwoLevelCache(
		"localhost:6379", // Test Redis address
		"",
		time.Minute*5,
	)

	pool := &types.Pool{
		Address:  "test-pool",
		Exchange: "Uniswap V2",
		Token0:   types.Token{Address: "0xtokena", Symbol: "TOKENA"},
		Token1:   types.Token{Address: "0xtokenb", Symbol: "TOKENB"},
		Reserve0: big.NewInt(1000000),
		Reserve1: big.NewInt(2000000),
	}

	// Test storage - since Redis might be unavailable, we mainly test local cache part
	err := tlc.StorePool(context.Background(), pool)
	// If Redis is unavailable, may return error, but we still verify local cache
	if err != nil {
		t.Logf("Redis store failed (expected in test environment): %v", err)
	}

	// Verify local cache has data
	localPool, err := tlc.localCache.GetPool(context.Background(), "test-pool")
	assert.NoError(t, err)
	assert.Equal(t, pool.Address, localPool.Address)
}

func TestTwoLevelCache_GetPool_LocalCacheHit(t *testing.T) {
	tlc := NewTwoLevelCache("localhost:6379", "", time.Minute*5)

	// First store data in local cache
	pool := &types.Pool{
		Address:  "local-pool",
		Exchange: "Uniswap V2",
	}
	tlc.localCache.StorePool(context.Background(), pool)

	// Test get - should hit local cache
	retrievedPool, err := tlc.GetPool(context.Background(), "local-pool")
	assert.NoError(t, err)
	assert.Equal(t, pool.Address, retrievedPool.Address)

	// Verify statistics
	stats := tlc.GetStats()
	assert.Equal(t, int64(1), stats.LocalHits)
}

func TestMemoryStore_BasicOperations(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	pool := &types.Pool{
		Address:  "test-pool",
		Exchange: "Uniswap V2",
		Token0:   types.Token{Address: "0xtokena", Symbol: "TOKENA"},
		Token1:   types.Token{Address: "0xtokenb", Symbol: "TOKENB"},
		Reserve0: big.NewInt(1000000),
		Reserve1: big.NewInt(2000000),
	}

	// Test storage
	err := store.StorePool(ctx, pool)
	assert.NoError(t, err)

	// Test retrieval
	retrievedPool, err := store.GetPool(ctx, "test-pool")
	assert.NoError(t, err)
	assert.Equal(t, pool.Address, retrievedPool.Address)

	// Test get by token pair
	pools, err := store.GetPoolsByTokens(ctx, "0xtokena", "0xtokenb")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(pools))

	// Test get all pools
	allPools, err := store.GetAllPools(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(allPools))
}

func TestMemoryStore_GetPool_NotFound(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	pool, err := store.GetPool(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Nil(t, pool)
}

func TestMemoryStore_TokenOperations(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	token := &types.Token{
		Address:  "0xtoken",
		Symbol:   "TEST",
		Decimals: 18,
	}

	// Store token (current implementation is no-op)
	err := store.StoreToken(ctx, token)
	assert.NoError(t, err)

	// Get token (returns default)
	retrievedToken, err := store.GetToken(ctx, "0xtoken")
	assert.NoError(t, err)
	assert.Equal(t, "UNKNOWN", retrievedToken.Symbol)
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	pool := &types.Pool{
		Address:  "concurrent-pool",
		Exchange: "Uniswap V2",
		Token0:   types.Token{Address: "0xtokena"},
		Token1:   types.Token{Address: "0xtokenb"},
		Reserve0: big.NewInt(1000000),
		Reserve1: big.NewInt(2000000),
	}

	// Concurrent storage
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			err := store.StorePool(ctx, pool)
			assert.NoError(t, err)
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify data consistency
	retrievedPool, err := store.GetPool(ctx, "concurrent-pool")
	assert.NoError(t, err)
	assert.Equal(t, pool.Address, retrievedPool.Address)
}

func TestMemoryStore_ConcurrentAccessSafe(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Create different pools to avoid duplicate storage issues
	pools := []*types.Pool{
		{
			Address:  "concurrent-pool-1",
			Exchange: "Uniswap V2",
			Token0:   types.Token{Address: "0xtokena1"},
			Token1:   types.Token{Address: "0xtokenb1"},
			Reserve0: big.NewInt(1000000),
			Reserve1: big.NewInt(2000000),
		},
		{
			Address:  "concurrent-pool-2",
			Exchange: "Uniswap V2",
			Token0:   types.Token{Address: "0xtokena2"},
			Token1:   types.Token{Address: "0xtokenb2"},
			Reserve0: big.NewInt(3000000),
			Reserve1: big.NewInt(4000000),
		},
		{
			Address:  "concurrent-pool-3",
			Exchange: "Uniswap V2",
			Token0:   types.Token{Address: "0xtokena3"},
			Token1:   types.Token{Address: "0xtokenb3"},
			Reserve0: big.NewInt(5000000),
			Reserve1: big.NewInt(6000000),
		},
	}

	// Concurrently store different pools
	done := make(chan bool, len(pools))
	for i, pool := range pools {
		go func(p *types.Pool, index int) {
			err := store.StorePool(ctx, p)
			assert.NoError(t, err)
			done <- true
		}(pool, i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < len(pools); i++ {
		<-done
	}

	// Verify all pools were stored correctly
	for _, pool := range pools {
		retrievedPool, err := store.GetPool(ctx, pool.Address)
		assert.NoError(t, err)
		assert.Equal(t, pool.Address, retrievedPool.Address)
	}

	// Verify total pool count
	allPools, err := store.GetAllPools(ctx)
	assert.NoError(t, err)
	assert.Equal(t, len(pools), len(allPools))
}
