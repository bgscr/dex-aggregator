package aggregator

import (
	"context"
	"log"
	"strings"

	"dex-aggregator/internal/cache"
	"dex-aggregator/internal/types"
)

type PathFinder struct {
	cache      cache.Store
	baseTokens []string
	maxHops    int
}

func NewPathFinder(cache cache.Store, baseTokens []string) *PathFinder {
	// 确保基础代币也是小写
	normalizedBaseTokens := make([]string, len(baseTokens))
	for i, token := range baseTokens {
		normalizedBaseTokens[i] = strings.ToLower(token)
	}

	return &PathFinder{
		cache:      cache,
		baseTokens: normalizedBaseTokens,
		maxHops:    3,
	}
}

// Find all possible paths
func (pf *PathFinder) FindAllPaths(ctx context.Context, tokenIn, tokenOut string, maxHops int) ([][]*types.Pool, error) {
	if maxHops == 0 {
		maxHops = pf.maxHops
	}

	var allPaths [][]*types.Pool

	// 统一转换为小写进行查找
	normalizedTokenIn := strings.ToLower(tokenIn)
	normalizedTokenOut := strings.ToLower(tokenOut)

	log.Printf("PathFinder: Looking for paths from %s to %s (normalized: %s to %s)",
		tokenIn, tokenOut, normalizedTokenIn, normalizedTokenOut)

	// 1. Direct paths (single hop)
	directPaths, err := pf.findDirectPaths(ctx, normalizedTokenIn, normalizedTokenOut)
	if err != nil {
		log.Printf("Error finding direct paths: %v", err)
	} else {
		log.Printf("Found %d direct paths", len(directPaths))
		allPaths = append(allPaths, directPaths...)
	}

	// 2. Two-hop paths through base tokens
	twoHopPaths, err := pf.findTwoHopPaths(ctx, normalizedTokenIn, normalizedTokenOut)
	if err != nil {
		log.Printf("Error finding two-hop paths: %v", err)
	} else {
		log.Printf("Found %d two-hop paths", len(twoHopPaths))
		allPaths = append(allPaths, twoHopPaths...)
	}

	log.Printf("Total paths found: %d", len(allPaths))
	return allPaths, nil
}

// Find direct paths
func (pf *PathFinder) findDirectPaths(ctx context.Context, tokenIn, tokenOut string) ([][]*types.Pool, error) {
	pools, err := pf.cache.GetPoolsByTokens(ctx, tokenIn, tokenOut)
	if err != nil {
		return nil, err
	}

	var paths [][]*types.Pool
	for _, pool := range pools {
		log.Printf("Direct path pool: %s (%s) with reserves %s/%s",
			pool.Address, pool.Exchange,
			pool.Reserve0.String(), pool.Reserve1.String())
		paths = append(paths, []*types.Pool{pool})
	}

	return paths, nil
}

// Find two-hop paths through base tokens
func (pf *PathFinder) findTwoHopPaths(ctx context.Context, tokenIn, tokenOut string) ([][]*types.Pool, error) {
	var paths [][]*types.Pool

	for _, baseToken := range pf.baseTokens {
		if baseToken == tokenIn || baseToken == tokenOut {
			continue
		}

		log.Printf("Checking two-hop path via base token: %s", baseToken)

		// First hop: tokenIn -> baseToken
		firstHopPools, _ := pf.cache.GetPoolsByTokens(ctx, tokenIn, baseToken)
		if len(firstHopPools) == 0 {
			log.Printf("No pools found for first hop: %s -> %s", tokenIn, baseToken)
			continue
		}

		// Second hop: baseToken -> tokenOut
		secondHopPools, _ := pf.cache.GetPoolsByTokens(ctx, baseToken, tokenOut)
		if len(secondHopPools) == 0 {
			log.Printf("No pools found for second hop: %s -> %s", baseToken, tokenOut)
			continue
		}

		// Combine paths
		for _, firstPool := range firstHopPools {
			for _, secondPool := range secondHopPools {
				path := []*types.Pool{firstPool, secondPool}
				paths = append(paths, path)
				log.Printf("Two-hop path found: %s -> %s -> %s",
					tokenIn, baseToken, tokenOut)
			}
		}
	}

	return paths, nil
}
