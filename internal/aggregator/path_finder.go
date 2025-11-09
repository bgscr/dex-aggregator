// dex-aggregator/internal/aggregator/path_finder.go

package aggregator

import (
	"container/heap"
	"context"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync"
	"time"

	"dex-aggregator/internal/cache"
	"dex-aggregator/internal/types"
)

type PathFinder struct {
	cache     cache.Store
	priceCalc *PriceCalculator // Add PriceCalculator dependency
	maxHops   int

	// Add in-memory cache for the graph
	graphLock    sync.RWMutex
	adj          map[string]map[string]bool
	poolMap      map[string]map[string][]*types.Pool
	liquidityMap map[string]map[string]*big.Int // Can also be kept for simple heuristics
}

// Update constructor
func NewPathFinder(cache cache.Store, priceCalc *PriceCalculator) *PathFinder {
	pf := &PathFinder{
		cache:        cache,
		priceCalc:    priceCalc, // Inject dependency
		maxHops:      3,
		adj:          make(map[string]map[string]bool),
		poolMap:      make(map[string]map[string][]*types.Pool),
		liquidityMap: make(map[string]map[string]*big.Int),
	}

	// 1. 在这里执行第一次、阻塞式的刷新
	// 这会延长服务器启动时间，但能保证服务启动后立即可用
	log.Println("PathFinder: Performing initial graph load...")
	if err := pf.RefreshGraph(context.Background()); err != nil {
		// 如果启动时无法加载图，服务将无法工作，这是一个致命错误
		log.Fatalf("PathFinder: Initial graph refresh failed: %v", err)
	}
	log.Println("PathFinder: Initial graph load complete.")

	refreshInterval := 30 * time.Second
	go pf.runGraphRefresher(context.Background(), refreshInterval)

	return pf
}

func (pf *PathFinder) runGraphRefresher(ctx context.Context, interval time.Duration) {
	log.Printf("PathFinder: Starting background graph refresher with %v interval", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Println("PathFinder: Periodic graph refresh triggered...")
			if err := pf.RefreshGraph(ctx); err != nil {
				log.Printf("PathFinder: Error during periodic graph refresh: %v", err)
			}
		case <-ctx.Done():
			log.Println("PathFinder: Stopping graph refresher.")
			return
		}
	}
}

// Graph refresh method
func (pf *PathFinder) RefreshGraph(ctx context.Context) error {
	log.Println("PathFinder: Refreshing graph from cache...")
	allPools, err := pf.cache.GetAllPools(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pools for graph refresh: %v", err)
	}

	// Build a new graph
	adj := make(map[string]map[string]bool)
	poolMap := make(map[string]map[string][]*types.Pool)
	liquidityMap := make(map[string]map[string]*big.Int)

	for _, pool := range allPools {
		t0 := strings.ToLower(pool.Token0.Address)
		t1 := strings.ToLower(pool.Token1.Address)

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

		adj[t0][t1] = true
		adj[t1][t0] = true
		poolMap[t0][t1] = append(poolMap[t0][t1], pool)
		poolMap[t1][t0] = append(poolMap[t1][t0], pool)

		poolLiquidity := new(big.Int).Mul(pool.Reserve0, pool.Reserve1)
		if existing, exists := liquidityMap[t0][t1]; exists {
			liquidityMap[t0][t1] = new(big.Int).Add(existing, poolLiquidity)
			liquidityMap[t1][t0] = new(big.Int).Add(existing, poolLiquidity)
		} else {
			liquidityMap[t0][t1] = poolLiquidity
			liquidityMap[t1][t0] = poolLiquidity
		}
	}

	// Thread-safely replace the old graph
	pf.graphLock.Lock()
	pf.adj = adj
	pf.poolMap = poolMap
	pf.liquidityMap = liquidityMap
	pf.graphLock.Unlock()

	log.Printf("PathFinder: Graph refreshed, %d pools loaded.", len(allPools))
	return nil
}

// --- Priority Queue Implementation ---

// pathState stores the state in the priority queue
type pathState struct {
	path      []*types.Pool // Path to this point (composed of Pools)
	amountOut *big.Int      // Amount of tokens held when reaching this point
	lastToken string        // Last token in this path
	index     int           // Index in the heap
}

// priorityQueue implements heap.Interface
type priorityQueue []*pathState

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	// We want a Max-Heap, so sort by amountOut in descending order
	return pq[i].amountOut.Cmp(pq[j].amountOut) > 0
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *priorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*pathState)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

// --- Override FindBestPaths ---

// FindBestPaths finds the optimal quote paths
func (pf *PathFinder) FindBestPaths(ctx context.Context, tokenIn, tokenOut string, amountIn *big.Int, maxHops, maxPaths int) ([][]*types.Pool, error) {
	if maxHops <= 0 {
		maxHops = pf.maxHops
	}

	normalizedTokenIn := strings.ToLower(tokenIn)
	normalizedTokenOut := strings.ToLower(tokenOut)

	log.Printf("PathFinder: Searching best paths from %s to %s (amountIn: %s, maxHops: %d, maxPaths: %d)",
		normalizedTokenIn, normalizedTokenOut, amountIn.String(), maxHops, maxPaths)

	// Read data from the cached graph (using read lock)
	pf.graphLock.RLock()
	defer pf.graphLock.RUnlock()

	if pf.adj[normalizedTokenIn] == nil {
		log.Printf("PathFinder: TokenIn %s not found in graph", normalizedTokenIn)
		return [][]*types.Pool{}, nil
	}
	if pf.adj[normalizedTokenOut] == nil {
		log.Printf("PathFinder: TokenOut %s not found in graph", normalizedTokenOut)
		return [][]*types.Pool{}, nil
	}

	var bestPaths [][]*types.Pool

	// Initialize Dijkstra
	// Priority queue, sorted by amountOut (max-heap)
	pq := make(priorityQueue, 0)
	heap.Init(&pq)

	// bestAmountPerToken records the highest output amount to reach a token, for pruning
	bestAmountPerToken := make(map[string]*big.Int)

	// Add all first-hop paths to the queue
	// Iterate over all neighbors of tokenIn
	for neighborToken := range pf.adj[normalizedTokenIn] {
		// Iterate over all pools between tokenIn and neighborToken
		for _, pool := range pf.poolMap[normalizedTokenIn][neighborToken] {
			// Simulate trade, calculate first hop output
			hopAmountOut, err := pf.priceCalc.CalculateOutput(pool, amountIn, normalizedTokenIn)
			if err != nil || hopAmountOut.Cmp(big.NewInt(0)) <= 0 {
				continue // Invalid trade or no output
			}

			newState := &pathState{
				path:      []*types.Pool{pool},
				amountOut: hopAmountOut,
				lastToken: neighborToken,
			}
			heap.Push(&pq, newState)

			if bestAmount, ok := bestAmountPerToken[neighborToken]; !ok || hopAmountOut.Cmp(bestAmount) > 0 {
				bestAmountPerToken[neighborToken] = hopAmountOut
			}
		}
	}

	// Start Dijkstra search
	for pq.Len() > 0 && len(bestPaths) < maxPaths {
		// Pop the path with the current maximum amountOut
		currentState := heap.Pop(&pq).(*pathState)

		// Check if it's a better path (pruning)
		// If we previously found a better quote to this token via a shorter (or same length) path, skip
		if bestAmount, ok := bestAmountPerToken[currentState.lastToken]; ok {
			if currentState.amountOut.Cmp(bestAmount) < 0 {
				continue
			}
		}

		// Check if destination is reached
		if currentState.lastToken == normalizedTokenOut {
			bestPaths = append(bestPaths, currentState.path)
			// Found a path, continue searching until maxPaths is met
			continue
		}

		// Check if maxHops is exceeded
		if len(currentState.path) >= maxHops {
			continue
		}

		// Explore neighbors (next hop)
		currentHopToken := currentState.lastToken
		currentHopAmountIn := currentState.amountOut

		for nextHopToken := range pf.adj[currentHopToken] {
			// Avoid loops (simple check)
			if pf.pathContainsToken(currentState.path, nextHopToken) {
				continue
			}

			// Iterate over all pools between currentHopToken and nextHopToken
			for _, pool := range pf.poolMap[currentHopToken][nextHopToken] {

				// Simulate trade
				nextHopAmountOut, err := pf.priceCalc.CalculateOutput(pool, currentHopAmountIn, currentHopToken)
				if err != nil || nextHopAmountOut.Cmp(big.NewInt(0)) <= 0 {
					continue
				}

				// Check if this is a better path to nextHopToken
				if bestAmount, ok := bestAmountPerToken[nextHopToken]; !ok || nextHopAmountOut.Cmp(bestAmount) > 0 {
					bestAmountPerToken[nextHopToken] = nextHopAmountOut

					// Create new path
					newPath := make([]*types.Pool, len(currentState.path)+1)
					copy(newPath, currentState.path)
					newPath[len(newPath)-1] = pool

					newState := &pathState{
						path:      newPath,
						amountOut: nextHopAmountOut,
						lastToken: nextHopToken,
					}
					heap.Push(&pq, newState)
				}
			}
		}
	}

	log.Printf("PathFinder: Found %d best paths.", len(bestPaths))
	return bestPaths, nil
}

// Helper function: check if path already contains a token (avoid loops)
func (pf *PathFinder) pathContainsToken(path []*types.Pool, token string) bool {
	// Simple loop check: check if the new token is already in the path (as either end of a pool)
	// Note: tokenIn is already in the previous hop
	for _, pool := range path {
		if strings.ToLower(pool.Token0.Address) == token || strings.ToLower(pool.Token1.Address) == token {
			return true
		}
	}
	return false
}
