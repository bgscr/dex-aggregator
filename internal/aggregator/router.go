package aggregator

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"sort"
	"strings"
	"sync"

	"dex-aggregator/internal/cache"
	"dex-aggregator/internal/types"
)

type Router struct {
	cache      cache.Store
	pathFinder *PathFinder
	calculator *PriceCalculator
}

func NewRouter(cache cache.Store) *Router {
	return &Router{
		cache:      cache,
		pathFinder: NewPathFinder(cache),
		calculator: NewPriceCalculator(),
	}
}

func (r *Router) GetBestQuote(ctx context.Context, req *types.QuoteRequest) (*types.QuoteResponse, error) {
	log.Printf("Starting quote: %s -> %s, amount: %s", req.TokenIn, req.TokenOut, req.AmountIn.String())

	tokenIn := strings.ToLower(req.TokenIn)
	tokenOut := strings.ToLower(req.TokenOut)

	paths, err := r.pathFinder.FindAllPaths(ctx, tokenIn, tokenOut, req.MaxHops)
	if err != nil {
		return nil, err
	}

	log.Printf("Found %d possible paths initially", len(paths))

	// --- 并发计算路径 ---
	var wg sync.WaitGroup
	// 使用带缓冲的 channel 来收集结果
	resultsChan := make(chan *types.TradePath, len(paths))

	for i, path := range paths {
		wg.Add(1) // 增加 WaitGroup 计数器

		// 启动 goroutine 进行计算
		// 必须将 path 和 i 作为参数传递，以避免循环变量捕获问题
		go func(p []*types.Pool, pathIndex int) {
			defer wg.Done() // 完成时减少计数器

			// log.Printf("Calculating path %d (contains %d pools)", pathIndex+1, len(p))

			amountOut, err := r.calculator.CalculatePathOutput(p, req.AmountIn, tokenIn, tokenOut)
			if err != nil {
				log.Printf("Failed to calculate path %d: %v", pathIndex+1, err)
				return // 终止此 goroutine
			}

			if amountOut.Cmp(big.NewInt(0)) <= 0 {
				// log.Printf("Path %d resulted in zero or negative output: %s", pathIndex+1, amountOut.String())
				return // 终止此 goroutine
			}

			gasCost := r.estimateGasCost(p)

			tradePath := &types.TradePath{
				Pools:     p,
				AmountOut: amountOut,
				Dexes:     r.getDexesFromPath(p),
				GasCost:   gasCost,
			}

			resultsChan <- tradePath // 将有效结果发送到 channel
			// log.Printf("Path %d output amount: %s", pathIndex+1, amountOut.String())

		}(path, i)
	}

	// 启动一个单独的 goroutine，在所有计算完成后关闭 channel
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// --- 收集所有结果 ---
	var tradePaths []*types.TradePath
	for tradePath := range resultsChan {
		tradePaths = append(tradePaths, tradePath)
	}
	// --- 并发计算结束 ---

	log.Printf("After calculation, found %d valid trade paths", len(tradePaths))

	if len(tradePaths) == 0 {
		return nil, fmt.Errorf("no valid path found")
	}

	// Sort by output amount (排序逻辑不变)
	sort.Slice(tradePaths, func(i, j int) bool {
		return tradePaths[i].AmountOut.Cmp(tradePaths[j].AmountOut) > 0
	})

	bestPath := tradePaths[0]

	// 考虑 Gas 成本 (逻辑不变)
	for i, path := range tradePaths {
		netAmount := new(big.Int).Sub(path.AmountOut, path.GasCost)
		currentBestNet := new(big.Int).Sub(bestPath.AmountOut, bestPath.GasCost)
		if netAmount.Cmp(currentBestNet) > 0 {
			bestPath = tradePaths[i]
		}
	}

	log.Printf("Best path output amount: %s", bestPath.AmountOut.String())

	return &types.QuoteResponse{
		AmountOut:   bestPath.AmountOut,
		Paths:       tradePaths,
		BestPath:    bestPath,
		GasEstimate: bestPath.GasCost,
	}, nil
}

// Estimate gas cost
func (r *Router) estimateGasCost(path []*types.Pool) *big.Int {
	// Simplified gas estimation: about 100k gas per trading pair
	return new(big.Int).Mul(big.NewInt(int64(len(path))), big.NewInt(100000))
}

// Get DEX names from path
func (r *Router) getDexesFromPath(path []*types.Pool) []string {
	dexes := make([]string, len(path))
	for i, pool := range path {
		dexes[i] = pool.Exchange
	}
	return dexes
}
