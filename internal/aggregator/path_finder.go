package aggregator

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"sort"
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

// FindBestPaths finds the most promising paths first before exploring all possibilities
func (pf *PathFinder) FindBestPaths(ctx context.Context, tokenIn, tokenOut string, amountIn *big.Int, maxHops, maxPaths int) ([][]*types.Pool, error) {
	if maxHops <= 0 {
		maxHops = pf.maxHops
	}

	normalizedTokenIn := strings.ToLower(tokenIn)
	normalizedTokenOut := strings.ToLower(tokenOut)

	log.Printf("PathFinder: Searching best paths from %s to %s (maxHops: %d, maxPaths: %d)",
		normalizedTokenIn, normalizedTokenOut, maxHops, maxPaths)

	allPools, err := pf.cache.GetAllPools(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get pools: %v", err)
	}

	log.Printf("PathFinder: Loaded %d pools from cache", len(allPools))

	// Build adjacency list and pool map with liquidity information
	adj := make(map[string]map[string]bool)
	poolMap := make(map[string]map[string][]*types.Pool)
	liquidityMap := make(map[string]map[string]*big.Int) // Track liquidity between tokens

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
		if liquidityMap[t0] == nil {
			liquidityMap[t0] = make(map[string]*big.Int)
		}
		if liquidityMap[t1] == nil {
			liquidityMap[t1] = make(map[string]*big.Int)
		}

		// Add edges in both directions
		adj[t0][t1] = true
		adj[t1][t0] = true

		// Add pools to poolMap
		poolMap[t0][t1] = append(poolMap[t0][t1], pool)
		poolMap[t1][t0] = append(poolMap[t1][t0], pool)

		// Calculate and accumulate liquidity
		poolLiquidity := new(big.Int).Mul(pool.Reserve0, pool.Reserve1)
		if existing, exists := liquidityMap[t0][t1]; exists {
			liquidityMap[t0][t1] = new(big.Int).Add(existing, poolLiquidity)
			liquidityMap[t1][t0] = new(big.Int).Add(existing, poolLiquidity)
		} else {
			liquidityMap[t0][t1] = poolLiquidity
			liquidityMap[t1][t0] = poolLiquidity
		}
	}

	// Check if start and end tokens exist in graph
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

	// Try direct paths first (most common and efficient)
	directPaths := pf.findDirectPaths(normalizedTokenIn, normalizedTokenOut, poolMap)
	if len(directPaths) >= maxPaths {
		log.Printf("PathFinder: Found %d direct paths, returning top %d", len(directPaths), maxPaths)
		return directPaths[:maxPaths], nil
	}

	// If not enough direct paths, find multi-hop paths with BFS but with liquidity priority
	allPaths := pf.findPathsWithPriority(ctx, normalizedTokenIn, normalizedTokenOut, maxHops, maxPaths, adj, poolMap, liquidityMap)

	log.Printf("PathFinder: Found %d total paths", len(allPaths))
	return allPaths, nil
}

// FindAllPaths finds all possible paths (original implementation with optimizations)
func (pf *PathFinder) FindAllPaths(ctx context.Context, tokenIn, tokenOut string, maxHops int) ([][]*types.Pool, error) {
	if maxHops <= 0 {
		maxHops = pf.maxHops
	}

	normalizedTokenIn := strings.ToLower(tokenIn)
	normalizedTokenOut := strings.ToLower(tokenOut)

	log.Printf("PathFinder: Searching all paths from %s to %s (maxHops: %d)",
		normalizedTokenIn, normalizedTokenOut, maxHops)

	allPools, err := pf.cache.GetAllPools(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get pools: %v", err)
	}

	log.Printf("PathFinder: Loaded %d pools from cache", len(allPools))

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

	// Check if start and end tokens exist in graph
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

	// BFS to find all token paths with cycle prevention
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
			continue
		}

		// Check max hops
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

// findDirectPaths finds direct paths between two tokens
func (pf *PathFinder) findDirectPaths(tokenIn, tokenOut string, poolMap map[string]map[string][]*types.Pool) [][]*types.Pool {
	var paths [][]*types.Pool

	pools := poolMap[tokenIn][tokenOut]
	for _, pool := range pools {
		paths = append(paths, []*types.Pool{pool})
	}

	return paths
}

// findPathsWithPriority finds paths prioritizing high liquidity routes
func (pf *PathFinder) findPathsWithPriority(ctx context.Context, tokenIn, tokenOut string, maxHops, maxPaths int,
	adj map[string]map[string]bool, poolMap map[string]map[string][]*types.Pool,
	liquidityMap map[string]map[string]*big.Int) [][]*types.Pool {

	var paths [][]*types.Pool
	queue := [][]string{{tokenIn}}
	visited := make(map[string]bool)
	visited[tokenIn] = true

	for len(queue) > 0 && len(paths) < maxPaths {
		currentPath := queue[0]
		queue = queue[1:]

		lastToken := currentPath[len(currentPath)-1]

		// Found target
		if lastToken == tokenOut {
			poolPaths := pf.buildPoolPaths(currentPath, poolMap)
			paths = append(paths, poolPaths...)
			continue
		}

		// Check max hops
		if len(currentPath) >= maxHops+1 {
			continue
		}

		// Get neighbors sorted by liquidity (highest first)
		neighbors := pf.getNeighborsByLiquidity(lastToken, adj, liquidityMap)
		for _, neighbor := range neighbors {
			// Avoid cycles
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

	return paths
}

// getNeighborsByLiquidity returns neighbors sorted by liquidity (highest first)
func (pf *PathFinder) getNeighborsByLiquidity(token string, adj map[string]map[string]bool, liquidityMap map[string]map[string]*big.Int) []string {
	var neighbors []string
	for neighbor := range adj[token] {
		neighbors = append(neighbors, neighbor)
	}

	// Sort by liquidity
	sort.Slice(neighbors, func(i, j int) bool {
		liquidityI := liquidityMap[token][neighbors[i]]
		liquidityJ := liquidityMap[token][neighbors[j]]

		// Handle nil liquidity (shouldn't happen but for safety)
		if liquidityI == nil {
			return false
		}
		if liquidityJ == nil {
			return true
		}

		return liquidityI.Cmp(liquidityJ) > 0
	})

	return neighbors
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
