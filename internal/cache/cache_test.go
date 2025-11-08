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
	// 使用真实的两级缓存，但使用内存存储模拟 Redis
	// 在实际环境中，这应该连接到测试 Redis 实例
	tlc := NewTwoLevelCache(
		"localhost:6379", // 测试 Redis 地址
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

	// 测试存储 - 由于 Redis 可能不可用，我们主要测试本地缓存部分
	err := tlc.StorePool(context.Background(), pool)
	// 如果 Redis 不可用，可能返回错误，但我们仍然验证本地缓存
	if err != nil {
		t.Logf("Redis store failed (expected in test environment): %v", err)
	}

	// 验证本地缓存中有数据
	localPool, err := tlc.localCache.GetPool(context.Background(), "test-pool")
	assert.NoError(t, err)
	assert.Equal(t, pool.Address, localPool.Address)
}

func TestTwoLevelCache_GetPool_LocalCacheHit(t *testing.T) {
	tlc := NewTwoLevelCache("localhost:6379", "", time.Minute*5)

	// 先在本地缓存中存储数据
	pool := &types.Pool{
		Address:  "local-pool",
		Exchange: "Uniswap V2",
	}
	tlc.localCache.StorePool(context.Background(), pool)

	// 测试获取 - 应该命中本地缓存
	retrievedPool, err := tlc.GetPool(context.Background(), "local-pool")
	assert.NoError(t, err)
	assert.Equal(t, pool.Address, retrievedPool.Address)

	// 验证统计信息
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

	// 测试存储
	err := store.StorePool(ctx, pool)
	assert.NoError(t, err)

	// 测试获取
	retrievedPool, err := store.GetPool(ctx, "test-pool")
	assert.NoError(t, err)
	assert.Equal(t, pool.Address, retrievedPool.Address)

	// 测试通过代币对获取
	pools, err := store.GetPoolsByTokens(ctx, "0xtokena", "0xtokenb")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(pools))

	// 测试获取所有池子
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

	// 存储 token（当前实现为空操作）
	err := store.StoreToken(ctx, token)
	assert.NoError(t, err)

	// 获取 token（返回默认值）
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

	// 并发存储
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			err := store.StorePool(ctx, pool)
			assert.NoError(t, err)
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证数据一致性
	retrievedPool, err := store.GetPool(ctx, "concurrent-pool")
	assert.NoError(t, err)
	assert.Equal(t, pool.Address, retrievedPool.Address)
}

func TestMemoryStore_ConcurrentAccessSafe(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// 创建不同的池子以避免重复存储问题
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

	// 并发存储不同的池子
	done := make(chan bool, len(pools))
	for i, pool := range pools {
		go func(p *types.Pool, index int) {
			err := store.StorePool(ctx, p)
			assert.NoError(t, err)
			done <- true
		}(pool, i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < len(pools); i++ {
		<-done
	}

	// 验证所有池子都被正确存储
	for _, pool := range pools {
		retrievedPool, err := store.GetPool(ctx, pool.Address)
		assert.NoError(t, err)
		assert.Equal(t, pool.Address, retrievedPool.Address)
	}

	// 验证总池子数量
	allPools, err := store.GetAllPools(ctx)
	assert.NoError(t, err)
	assert.Equal(t, len(pools), len(allPools))
}
