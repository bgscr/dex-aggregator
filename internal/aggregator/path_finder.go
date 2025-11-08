package aggregator

import (
	"context"
	"fmt"
	"log"
	"strings"

	"dex-aggregator/internal/cache"
	"dex-aggregator/internal/types"
)

type PathFinder struct {
	cache   cache.Store
	maxHops int
}

func NewPathFinder(cache cache.Store) *PathFinder {
	return &PathFinder{
		cache:   cache,
		maxHops: 3,
	}
}

func (pf *PathFinder) FindAllPaths(ctx context.Context, tokenIn, tokenOut string, maxHops int) ([][]*types.Pool, error) {
	if maxHops <= 0 {
		maxHops = pf.maxHops
	}

	normalizedTokenIn := strings.ToLower(tokenIn)
	normalizedTokenOut := strings.ToLower(tokenOut)

	log.Printf("PathFinder: Searching paths from %s to %s (maxHops: %d)",
		normalizedTokenIn, normalizedTokenOut, maxHops)

	allPools, err := pf.cache.GetAllPools(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get pools: %v", err)
	}

	log.Printf("PathFinder: Loaded %d pools from cache", len(allPools))

	// Debug: print all token addresses in pools
	for i, pool := range allPools {
		if i < 5 { // Print first 5 pools for debugging
			log.Printf("Pool %d: %s <-> %s (%s/%s)", i,
				pool.Token0.Address, pool.Token1.Address,
				pool.Token0.Symbol, pool.Token1.Symbol)
		}
	}

	// Build adjacency list and pool map
	adj := make(map[string]map[string]bool)
	poolMap := make(map[string]map[string][]*types.Pool)

	for _, pool := range allPools {
		t0 := strings.ToLower(pool.Token0.Address)
		t1 := strings.ToLower(pool.Token1.Address)

		// Initialize maps if needed
		if adj[t0] == nil {
			adj[t0] = make(map[string]bool)
		}
		if adj[t1] == nil {
			adj[t1] = make(map[string]bool)
		}
		if poolMap[t0] == nil {
			poolMap[t0] = make(map[string][]*types.Pool)
		}
		if poolMap[t1] == nil {
			poolMap[t1] = make(map[string][]*types.Pool)
		}

		// Add edges in both directions
		adj[t0][t1] = true
		adj[t1][t0] = true

		// Add pools to poolMap
		poolMap[t0][t1] = append(poolMap[t0][t1], pool)
		poolMap[t1][t0] = append(poolMap[t1][t0], pool)
	}

	// Debug: check if start and end tokens exist in graph
	if adj[normalizedTokenIn] == nil {
		log.Printf("PathFinder: TokenIn %s not found in graph", normalizedTokenIn)
		return [][]*types.Pool{}, nil
	}
	if adj[normalizedTokenOut] == nil {
		log.Printf("PathFinder: TokenOut %s not found in graph", normalizedTokenOut)
		return [][]*types.Pool{}, nil
	}

	log.Printf("PathFinder: TokenIn %s has %d neighbors", normalizedTokenIn, len(adj[normalizedTokenIn]))
	log.Printf("PathFinder: TokenOut %s has %d neighbors", normalizedTokenOut, len(adj[normalizedTokenOut]))

	// BFS to find all token paths
	var allTokenPaths [][]string
	queue := [][]string{{normalizedTokenIn}}
	visited := make(map[string]bool)
	visited[normalizedTokenIn] = true

	for len(queue) > 0 {
		currentPath := queue[0]
		queue = queue[1:]

		lastToken := currentPath[len(currentPath)-1]

		// Found target
		if lastToken == normalizedTokenOut {
			allTokenPaths = append(allTokenPaths, currentPath)
			continue // Continue to find all paths, not just the first one
		}

		// Check max hops (path length = hops + 1)
		if len(currentPath) >= maxHops+1 {
			continue
		}

		// Explore neighbors
		for neighbor := range adj[lastToken] {
			// Check if neighbor is already in current path to avoid cycles
			if containsToken(currentPath, neighbor) {
				continue
			}

			// Create new path
			newPath := make([]string, len(currentPath))
			copy(newPath, currentPath)
			newPath = append(newPath, neighbor)
			queue = append(queue, newPath)
		}
	}

	log.Printf("PathFinder: Found %d token paths", len(allTokenPaths))

	// Convert token paths to pool paths
	var allPoolPaths [][]*types.Pool
	for _, tokenPath := range allTokenPaths {
		poolPaths := pf.buildPoolPaths(tokenPath, poolMap)
		allPoolPaths = append(allPoolPaths, poolPaths...)
	}

	log.Printf("PathFinder: Found %d total pool paths", len(allPoolPaths))
	return allPoolPaths, nil
}

func (pf *PathFinder) buildPoolPaths(tokens []string, poolMap map[string]map[string][]*types.Pool) [][]*types.Pool {
	var paths [][]*types.Pool
	var currentPath []*types.Pool

	var backtrack func(int)
	backtrack = func(tokenIndex int) {
		if tokenIndex == len(tokens)-1 {
			finalPath := make([]*types.Pool, len(currentPath))
			copy(finalPath, currentPath)
			paths = append(paths, finalPath)
			return
		}

		tokenA := tokens[tokenIndex]
		tokenB := tokens[tokenIndex+1]

		availablePools := poolMap[tokenA][tokenB]
		if len(availablePools) == 0 {
			log.Printf("PathFinder: No pools found for %s -> %s", tokenA, tokenB)
			return
		}

		for _, pool := range availablePools {
			currentPath = append(currentPath, pool)
			backtrack(tokenIndex + 1)
			currentPath = currentPath[:len(currentPath)-1]
		}
	}

	backtrack(0)
	return paths
}

func containsToken(tokens []string, token string) bool {
	for _, t := range tokens {
		if t == token {
			return true
		}
	}
	return false
}
