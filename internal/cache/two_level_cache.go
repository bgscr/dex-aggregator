package cache

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"dex-aggregator/internal/types"
)

// TwoLevelCache provides caching with both memory and Redis layers
type TwoLevelCache struct {
	localCache *MemoryStore
	redisCache *RedisStore
	localTTL   time.Duration
	mutex      sync.RWMutex
	stats      *CacheStats
}

// CacheStats tracks cache performance metrics
type CacheStats struct {
	LocalHits   int64
	LocalMisses int64
	RedisHits   int64
	RedisMisses int64
	mutex       sync.RWMutex
}

func NewTwoLevelCache(redisAddr, redisPassword string, localTTL time.Duration) *TwoLevelCache {
	return &TwoLevelCache{
		localCache: NewMemoryStore(),
		redisCache: NewRedisStore(redisAddr, redisPassword),
		localTTL:   localTTL,
		stats:      &CacheStats{},
	}
}

// StorePool stores pool in both cache layers
func (tlc *TwoLevelCache) StorePool(ctx context.Context, pool *types.Pool) error {
	// Store in local cache
	if err := tlc.localCache.StorePool(ctx, pool); err != nil {
		log.Printf("Warning: Failed to store pool in local cache: %v", err)
	}

	// Store in Redis
	if err := tlc.redisCache.StorePool(ctx, pool); err != nil {
		return fmt.Errorf("failed to store pool in Redis: %v", err)
	}

	return nil
}

// GetPool retrieves pool with two-level cache lookup
func (tlc *TwoLevelCache) GetPool(ctx context.Context, address string) (*types.Pool, error) {
	// First try local cache
	pool, err := tlc.localCache.GetPool(ctx, address)
	if err == nil {
		tlc.stats.mutex.Lock()
		tlc.stats.LocalHits++
		tlc.stats.mutex.Unlock()
		return pool, nil
	}

	tlc.stats.mutex.Lock()
	tlc.stats.LocalMisses++
	tlc.stats.mutex.Unlock()

	// Local cache miss, try Redis
	pool, err = tlc.redisCache.GetPool(ctx, address)
	if err != nil {
		tlc.stats.mutex.Lock()
		tlc.stats.RedisMisses++
		tlc.stats.mutex.Unlock()
		return nil, err
	}

	tlc.stats.mutex.Lock()
	tlc.stats.RedisHits++
	tlc.stats.mutex.Unlock()

	// Populate local cache
	go func() {
		// Use background context to avoid cancellation
		bgCtx := context.Background()
		if err := tlc.localCache.StorePool(bgCtx, pool); err != nil {
			log.Printf("Warning: Failed to backfill local cache: %v", err)
		}
	}()

	return pool, nil
}

// GetAllPools gets all pools with caching optimization
func (tlc *TwoLevelCache) GetAllPools(ctx context.Context) ([]*types.Pool, error) {
	// For getAll operations, always use Redis as the source of truth
	pools, err := tlc.redisCache.GetAllPools(ctx)
	if err != nil {
		return nil, err
	}

	// Update local cache in background
	go tlc.warmLocalCache(pools)

	return pools, nil
}

// warmLocalCache updates local cache with fresh data
func (tlc *TwoLevelCache) warmLocalCache(pools []*types.Pool) {
	bgCtx := context.Background()
	for _, pool := range pools {
		if err := tlc.localCache.StorePool(bgCtx, pool); err != nil {
			log.Printf("Warning: Failed to warm local cache: %v", err)
		}
	}
}

// GetPoolsByTokens searches pools by token pair
func (tlc *TwoLevelCache) GetPoolsByTokens(ctx context.Context, tokenA, tokenB string) ([]*types.Pool, error) {
	// For token pair searches, use Redis directly as memory store doesn't have efficient indexing
	return tlc.redisCache.GetPoolsByTokens(ctx, tokenA, tokenB)
}

// StoreToken stores token information
func (tlc *TwoLevelCache) StoreToken(ctx context.Context, token *types.Token) error {
	// Store in both caches
	if err := tlc.localCache.StoreToken(ctx, token); err != nil {
		log.Printf("Warning: Failed to store token in local cache: %v", err)
	}

	return tlc.redisCache.StoreToken(ctx, token)
}

// GetToken retrieves token information
func (tlc *TwoLevelCache) GetToken(ctx context.Context, address string) (*types.Token, error) {
	// Try local cache first
	token, err := tlc.localCache.GetToken(ctx, address)
	if err == nil {
		return token, nil
	}

	// Fall back to Redis
	return tlc.redisCache.GetToken(ctx, address)
}

// GetStats returns cache performance statistics
func (tlc *TwoLevelCache) GetStats() *CacheStats {
	tlc.stats.mutex.RLock()
	defer tlc.stats.mutex.RUnlock()

	return &CacheStats{
		LocalHits:   tlc.stats.LocalHits,
		LocalMisses: tlc.stats.LocalMisses,
		RedisHits:   tlc.stats.RedisHits,
		RedisMisses: tlc.stats.RedisMisses,
	}
}

// ClearLocalCache clears the local memory cache
func (tlc *TwoLevelCache) ClearLocalCache() {
	// Implementation would require adding Clear method to MemoryStore
	log.Println("Local cache clear requested - would need MemoryStore enhancement")
}
