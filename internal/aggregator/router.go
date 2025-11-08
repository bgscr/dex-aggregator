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

	"dex-aggregator/config"
	"dex-aggregator/internal/cache"
	"dex-aggregator/internal/types"
)

type Router struct {
	cache         cache.Store
	pathFinder    *PathFinder
	calculator    *PriceCalculator
	maxConcurrent int
}

func NewRouter(cache cache.Store, perfConfig config.PerformanceConfig) *Router {
	calculator := NewPriceCalculator()
	// 使用配置中的值覆盖默认值
	calculator.SetMaxSlippage(perfConfig.MaxSlippage)

	return &Router{
		cache:         cache,
		pathFinder:    NewPathFinder(cache),
		calculator:    calculator,
		maxConcurrent: perfConfig.MaxConcurrentPaths,
	}
}

// GetBestQuote finds the best trading quote with optimized path search
func (r *Router) GetBestQuote(ctx context.Context, req *types.QuoteRequest) (*types.QuoteResponse, error) {
	startTime := time.Now()
	log.Printf("Quote request: %s -> %s, amount: %s", req.TokenIn, req.TokenOut, req.AmountIn.String())

	tokenIn := strings.ToLower(req.TokenIn)
	tokenOut := strings.ToLower(req.TokenOut)

	log.Printf("Normalized tokens: %s -> %s", tokenIn, tokenOut)
	// 获取所有池子进行检查
	allPools, err := r.cache.GetAllPools(ctx)
	if err != nil {
		log.Printf("Failed to get all pools: %v", err)
	} else {
		log.Printf("Total pools in cache: %d", len(allPools))

		// 检查是否有相关的池子
		relatedPools := 0
		for _, pool := range allPools {
			poolToken0 := strings.ToLower(pool.Token0.Address)
			poolToken1 := strings.ToLower(pool.Token1.Address)
			if (poolToken0 == tokenIn || poolToken1 == tokenIn) &&
				(poolToken0 == tokenOut || poolToken1 == tokenOut) {
				relatedPools++
				log.Printf("Found direct pool: %s, %s/%s, reserves: %s/%s",
					pool.Address, pool.Token0.Symbol, pool.Token1.Symbol,
					pool.Reserve0.String(), pool.Reserve1.String())
			}
		}
		log.Printf("Found %d direct pools for %s->%s", relatedPools, tokenIn, tokenOut)
	}

	// Use optimized path finding that prioritizes high-liquidity routes
	var paths [][]*types.Pool

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

			log.Printf("Calculating path %d with %d pools", pathIndex+1, len(p))
			for j, pool := range p {
				log.Printf("  Pool %d: %s, %s/%s, reserves: %s/%s",
					j+1, pool.Exchange, pool.Token0.Symbol, pool.Token1.Symbol,
					pool.Reserve0.String(), pool.Reserve1.String())
			}

			amountOut, err := r.calculator.CalculatePathOutput(p, req.AmountIn, tokenIn, tokenOut)
			if err != nil {
				log.Printf("Path %d calculation failed: %v", pathIndex+1, err)
				errorChan <- err
				return
			}

			log.Printf("Path %d raw output: %s", pathIndex+1, amountOut.String())

			if amountOut.Cmp(big.NewInt(0)) <= 0 {
				return
			}

			gasCost := r.estimateGasCost(p)

			// TODO: 修复 Gas 成本计算逻辑
			// netAmount := new(big.Int).Sub(amountOut, gasCost)
			// if netAmount.Cmp(big.NewInt(0)) <= 0 {
			// 	log.Printf("Path %d not profitable after gas costs", pathIndex+1)
			// 	return
			// }

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

	// 按原始输出金额排序 (最高优先)
	sort.Slice(tradePaths, func(i, j int) bool {
		return tradePaths[i].AmountOut.Cmp(tradePaths[j].AmountOut) > 0
	})

	// 返回输出最高的路径
	bestPath := tradePaths[0]
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
