package cache

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync"

	"dex-aggregator/internal/types"
)

// MemoryStore in-memory storage implementation
type MemoryStore struct {
	pools      map[string]*types.Pool
	tokenPairs map[string]map[string][]string // tokenA -> tokenB -> []poolAddress
	mutex      sync.RWMutex
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		pools:      make(map[string]*types.Pool),
		tokenPairs: make(map[string]map[string][]string),
	}
}

func (ms *MemoryStore) StorePool(ctx context.Context, pool *types.Pool) error {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	// Ensure reserves are properly initialized
	if pool.Reserve0 == nil {
		pool.Reserve0 = big.NewInt(0)
	}
	if pool.Reserve1 == nil {
		pool.Reserve1 = big.NewInt(0)
	}

	// Store pool
	ms.pools[pool.Address] = pool

	// Create token pair index (normalize addresses to lowercase)
	token0 := strings.ToLower(pool.Token0.Address)
	token1 := strings.ToLower(pool.Token1.Address)

	log.Printf("Storing pool: %s, Tokens: %s(%s) / %s(%s)",
		pool.Address, pool.Token0.Symbol, token0, pool.Token1.Symbol, token1)

	if ms.tokenPairs[token0] == nil {
		ms.tokenPairs[token0] = make(map[string][]string)
	}
	if ms.tokenPairs[token1] == nil {
		ms.tokenPairs[token1] = make(map[string][]string)
	}

	ms.tokenPairs[token0][token1] = append(ms.tokenPairs[token0][token1], pool.Address)
	ms.tokenPairs[token1][token0] = append(ms.tokenPairs[token1][token0], pool.Address)

	log.Printf("Created index: %s<->%s -> %v", token0, token1, ms.tokenPairs[token0][token1])

	return nil
}

func (ms *MemoryStore) GetPool(ctx context.Context, address string) (*types.Pool, error) {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()

	pool, exists := ms.pools[address]
	if !exists {
		return nil, fmt.Errorf("pool not found")
	}

	return pool, nil
}

func (ms *MemoryStore) GetPoolsByTokens(ctx context.Context, tokenA, tokenB string) ([]*types.Pool, error) {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()

	// 确保传入的参数已经是小写，但这里再确认一次
	tokenA = strings.ToLower(tokenA)
	tokenB = strings.ToLower(tokenB)

	log.Printf("Cache lookup for tokens: %s <-> %s", tokenA, tokenB)
	log.Printf("Available token pairs in cache: %v", ms.getAvailableTokenPairs())

	var pools []*types.Pool

	// Find pools for tokenA -> tokenB
	if pairs, ok := ms.tokenPairs[tokenA]; ok {
		if poolAddrs, ok := pairs[tokenB]; ok {
			for _, addr := range poolAddrs {
				if pool, exists := ms.pools[addr]; exists {
					pools = append(pools, pool)
				}
			}
		}
	}

	log.Printf("Found %d pools for token pair %s/%s", len(pools), tokenA, tokenB)

	return pools, nil
}

// Helper method to get available token pairs for debugging
func (ms *MemoryStore) getAvailableTokenPairs() []string {
	var pairs []string
	for tokenA, subPairs := range ms.tokenPairs {
		for tokenB := range subPairs {
			pairs = append(pairs, fmt.Sprintf("%s-%s", tokenA, tokenB))
		}
	}
	return pairs
}

func (ms *MemoryStore) GetAllPools(ctx context.Context) ([]*types.Pool, error) {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()

	var pools []*types.Pool
	for _, pool := range ms.pools {
		pools = append(pools, pool)
	}

	return pools, nil
}

func (ms *MemoryStore) StoreToken(ctx context.Context, token *types.Token) error {
	return nil
}

func (ms *MemoryStore) GetToken(ctx context.Context, address string) (*types.Token, error) {
	return &types.Token{
		Address:  address,
		Symbol:   "UNKNOWN",
		Decimals: 18,
	}, nil
}
