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
	log.Printf("Quote request: %s -> %s, amount: %s", req.TokenIn, req.TokenOut, req.AmountIn.String())

	tokenIn := strings.ToLower(req.TokenIn)
	tokenOut := strings.ToLower(req.TokenOut)

	paths, err := r.pathFinder.FindAllPaths(ctx, tokenIn, tokenOut, req.MaxHops)
	if err != nil {
		return nil, err
	}

	log.Printf("Found %d possible paths", len(paths))

	var wg sync.WaitGroup
	resultsChan := make(chan *types.TradePath, len(paths))

	for i, path := range paths {
		wg.Add(1)

		go func(p []*types.Pool, pathIndex int) {
			defer wg.Done()

			amountOut, err := r.calculator.CalculatePathOutput(p, req.AmountIn, tokenIn, tokenOut)
			if err != nil {
				log.Printf("Path %d calculation failed: %v", pathIndex+1, err)
				return
			}

			if amountOut.Cmp(big.NewInt(0)) <= 0 {
				return
			}

			gasCost := r.estimateGasCost(p)

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
	}()

	var tradePaths []*types.TradePath
	for tradePath := range resultsChan {
		tradePaths = append(tradePaths, tradePath)
	}

	log.Printf("After calculation, found %d valid trade paths", len(tradePaths))

	if len(tradePaths) == 0 {
		return nil, fmt.Errorf("no valid path found")
	}

	sort.Slice(tradePaths, func(i, j int) bool {
		return tradePaths[i].AmountOut.Cmp(tradePaths[j].AmountOut) > 0
	})

	bestPath := tradePaths[0]

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

func (r *Router) estimateGasCost(path []*types.Pool) *big.Int {
	return new(big.Int).Mul(big.NewInt(int64(len(path))), big.NewInt(100000))
}

func (r *Router) getDexesFromPath(path []*types.Pool) []string {
	dexes := make([]string, len(path))
	for i, pool := range path {
		dexes[i] = pool.Exchange
	}
	return dexes
}
