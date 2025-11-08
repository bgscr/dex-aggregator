package aggregator

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"sort"
	"strings"
	"sync"
	"time"

	"dex-aggregator/internal/cache"
	"dex-aggregator/internal/types"
)

type Router struct {
	cache         cache.Store
	pathFinder    *PathFinder
	calculator    *PriceCalculator
	maxConcurrent int
}

func NewRouter(cache cache.Store) *Router {
	return &Router{
		cache:         cache,
		pathFinder:    NewPathFinder(cache),
		calculator:    NewPriceCalculator(),
		maxConcurrent: 10, // Limit concurrent path calculations
	}
}

// GetBestQuote finds the best trading quote with optimized path search
func (r *Router) GetBestQuote(ctx context.Context, req *types.QuoteRequest) (*types.QuoteResponse, error) {
	startTime := time.Now()
	log.Printf("Quote request: %s -> %s, amount: %s", req.TokenIn, req.TokenOut, req.AmountIn.String())

	tokenIn := strings.ToLower(req.TokenIn)
	tokenOut := strings.ToLower(req.TokenOut)

	// Use optimized path finding that prioritizes high-liquidity routes
	var paths [][]*types.Pool
	var err error

	if req.MaxHops == 0 {
		req.MaxHops = 3
	}

	// For large amounts, be more selective with paths to reduce computation
	maxPaths := 20
	if req.AmountIn.Cmp(big.NewInt(1000000000000000000)) > 0 { // > 1 ETH
		maxPaths = 10
	}

	paths, err = r.pathFinder.FindBestPaths(ctx, tokenIn, tokenOut, req.AmountIn, req.MaxHops, maxPaths)
	if err != nil {
		return nil, err
	}

	log.Printf("Found %d possible paths in %v", len(paths), time.Since(startTime))

	if len(paths) == 0 {
		return nil, fmt.Errorf("no valid path found")
	}

	// Calculate outputs for all paths with concurrency control
	tradePaths := r.calculatePathsConcurrently(ctx, paths, req, tokenIn, tokenOut)

	log.Printf("After calculation, found %d valid trade paths in %v", len(tradePaths), time.Since(startTime))

	if len(tradePaths) == 0 {
		return nil, fmt.Errorf("no valid path with positive output found")
	}

	// Find the best path considering both output amount and gas costs
	bestPath := r.findOptimalPath(tradePaths)

	log.Printf("Best path output amount: %s (net: %s after gas)",
		bestPath.AmountOut.String(),
		new(big.Int).Sub(bestPath.AmountOut, bestPath.GasCost).String())

	totalTime := time.Since(startTime)
	log.Printf("Total quote processing time: %v", totalTime)

	return &types.QuoteResponse{
		AmountOut:      bestPath.AmountOut,
		Paths:          tradePaths,
		BestPath:       bestPath,
		GasEstimate:    bestPath.GasCost,
		ProcessingTime: totalTime.Milliseconds(),
	}, nil
}

// calculatePathsConcurrently processes paths with controlled concurrency
func (r *Router) calculatePathsConcurrently(ctx context.Context, paths [][]*types.Pool, req *types.QuoteRequest, tokenIn, tokenOut string) []*types.TradePath {
	var wg sync.WaitGroup
	sem := make(chan struct{}, r.maxConcurrent) // Semaphore for limiting concurrency
	resultsChan := make(chan *types.TradePath, len(paths))
	errorChan := make(chan error, len(paths))

	for i, path := range paths {
		wg.Add(1)

		go func(p []*types.Pool, pathIndex int) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			amountOut, err := r.calculator.CalculatePathOutput(p, req.AmountIn, tokenIn, tokenOut)
			if err != nil {
				log.Printf("Path %d calculation failed: %v", pathIndex+1, err)
				errorChan <- err
				return
			}

			if amountOut.Cmp(big.NewInt(0)) <= 0 {
				return
			}

			gasCost := r.estimateGasCost(p)

			// Check if the path is profitable after gas
			netAmount := new(big.Int).Sub(amountOut, gasCost)
			if netAmount.Cmp(big.NewInt(0)) <= 0 {
				log.Printf("Path %d not profitable after gas costs", pathIndex+1)
				return
			}

			tradePath := &types.TradePath{
				Pools:     p,
				AmountOut: amountOut,
				Dexes:     r.getDexesFromPath(p),
				GasCost:   gasCost,
			}

			resultsChan <- tradePath
		}(path, i)
	}

	go func() {
		wg.Wait()
		close(resultsChan)
		close(errorChan)
	}()

	var tradePaths []*types.TradePath
	for tradePath := range resultsChan {
		tradePaths = append(tradePaths, tradePath)
	}

	// Log any errors that occurred
	var errorCount int
	for range errorChan {
		errorCount++
	}
	if errorCount > 0 {
		log.Printf("%d paths had calculation errors", errorCount)
	}

	return tradePaths
}

// findOptimalPath finds the best path considering both output and gas costs
func (r *Router) findOptimalPath(tradePaths []*types.TradePath) *types.TradePath {
	if len(tradePaths) == 0 {
		return nil
	}

	// Sort by raw output amount first
	sort.Slice(tradePaths, func(i, j int) bool {
		return tradePaths[i].AmountOut.Cmp(tradePaths[j].AmountOut) > 0
	})

	bestPath := tradePaths[0]

	// Then check if any path has better net amount (after gas)
	for i := 1; i < len(tradePaths) && i < 5; i++ { // Only check top 5 to avoid micro-optimization
		currentNet := new(big.Int).Sub(tradePaths[i].AmountOut, tradePaths[i].GasCost)
		bestNet := new(big.Int).Sub(bestPath.AmountOut, bestPath.GasCost)

		if currentNet.Cmp(bestNet) > 0 {
			bestPath = tradePaths[i]
		}
	}

	return bestPath
}

// estimateGasCost provides more accurate gas estimation based on DEX type
func (r *Router) estimateGasCost(path []*types.Pool) *big.Int {
	baseGas := big.NewInt(21000) // Base transaction gas
	swapGas := big.NewInt(0)

	for _, pool := range path {
		// Different DEXes have different gas costs
		var poolGas int64
		switch strings.ToLower(pool.Exchange) {
		case "uniswap v2":
			poolGas = 100000
		case "sushiswap":
			poolGas = 120000
		default:
			poolGas = 110000 // Default estimate
		}
		swapGas.Add(swapGas, big.NewInt(poolGas))
	}

	return new(big.Int).Add(baseGas, swapGas)
}

func (r *Router) getDexesFromPath(path []*types.Pool) []string {
	dexes := make([]string, len(path))
	for i, pool := range path {
		dexes[i] = pool.Exchange
	}
	return dexes
}
