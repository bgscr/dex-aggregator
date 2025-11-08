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

type Router struct {
	cache      cache.Store
	pathFinder *PathFinder
	calculator *PriceCalculator
	baseTokens []string
}

func NewRouter(cache cache.Store, baseTokens []string) *Router {
	return &Router{
		cache:      cache,
		pathFinder: NewPathFinder(cache, baseTokens),
		calculator: NewPriceCalculator(),
		baseTokens: baseTokens,
	}
}

func (r *Router) GetBestQuote(ctx context.Context, req *types.QuoteRequest) (*types.QuoteResponse, error) {
	log.Printf("Starting quote: %s -> %s, amount: %s", req.TokenIn, req.TokenOut, req.AmountIn.String())

	// 2. 在这里将请求中的地址转换为小写
	tokenIn := strings.ToLower(req.TokenIn)
	tokenOut := strings.ToLower(req.TokenOut)

	// 3. 使用转换后的小写地址进行路径查找
	paths, err := r.pathFinder.FindAllPaths(ctx, tokenIn, tokenOut, req.MaxHops)
	if err != nil {
		return nil, err
	}

	log.Printf("Found %d possible paths initially", len(paths))

	var tradePaths []*types.TradePath
	for i, path := range paths {
		log.Printf("Calculating path %d (contains %d pools)", i+1, len(path))

		// 4. 使用转换后的小写地址进行价格计算
		amountOut, err := r.calculator.CalculatePathOutput(path, req.AmountIn, tokenIn, tokenOut)
		if err != nil {
			log.Printf("Failed to calculate path %d: %v", i+1, err)
			continue
		}

		if amountOut.Cmp(big.NewInt(0)) <= 0 {
			log.Printf("Path %d resulted in zero or negative output: %s", i+1, amountOut.String())
			continue
		}

		gasCost := r.estimateGasCost(path)

		tradePath := &types.TradePath{
			Pools:     path,
			AmountOut: amountOut,
			Dexes:     r.getDexesFromPath(path),
			GasCost:   gasCost,
		}

		tradePaths = append(tradePaths, tradePath)
		log.Printf("Path %d output amount: %s", i+1, amountOut.String())
	}

	log.Printf("After calculation, found %d valid trade paths", len(tradePaths))

	if len(tradePaths) == 0 {
		return nil, fmt.Errorf("no valid path found")
	}

	// Sort by output amount
	sort.Slice(tradePaths, func(i, j int) bool {
		return tradePaths[i].AmountOut.Cmp(tradePaths[j].AmountOut) > 0
	})

	bestPath := tradePaths[0]

	// Consider net profit after gas cost
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
