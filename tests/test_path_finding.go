package main

import (
	"context"
	"fmt"
	"log"

	"dex-aggregator/internal/aggregator"
	"dex-aggregator/internal/cache"
	"dex-aggregator/internal/collector"
)

func TestPathFindingDetails() {
	fmt.Println("=== Testing Path Finding Details ===")

	store := cache.NewMemoryStore()
	poolCollector := collector.NewMockPoolCollector(store)
	poolCollector.InitMockPools()

	pathFinder := aggregator.NewPathFinder(store)
	ctx := context.Background()

	testCases := []struct {
		name     string
		tokenIn  string
		tokenOut string
		maxHops  int
	}{
		{
			name:     "Direct path",
			tokenIn:  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
			tokenOut: "0xdac17f958d2ee523a2206206994597c13d831ec7",
			maxHops:  1,
		},
		{
			name:     "2-hop path",
			tokenIn:  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
			tokenOut: "0x6b175474e89094c44da98b954eedeac495271d0f",
			maxHops:  2,
		},
		{
			name:     "3-hop max",
			tokenIn:  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
			tokenOut: "0x6b175474e89094c44da98b954eedeac495271d0f",
			maxHops:  3,
		},
	}

	for _, tc := range testCases {
		fmt.Printf("\nTesting: %s\n", tc.name)
		fmt.Printf("  From: %s\n", tc.tokenIn)
		fmt.Printf("  To: %s\n", tc.tokenOut)
		fmt.Printf("  Max Hops: %d\n", tc.maxHops)

		paths, err := pathFinder.FindAllPaths(ctx, tc.tokenIn, tc.tokenOut, tc.maxHops)
		if err != nil {
			log.Printf("  Path finding failed: %v", err)
			continue
		}

		fmt.Printf("  Found %d total paths\n", len(paths))

		// 显示前3个路径的详细信息
		for i, path := range paths {
			if i >= 3 {
				fmt.Printf("  ... and %d more paths\n", len(paths)-3)
				break
			}

			fmt.Printf("  Path %d: ", i+1)
			for j, pool := range path {
				if j > 0 {
					fmt.Printf(" → ")
				}
				fmt.Printf("%s[%s/%s]", pool.Exchange, pool.Token0.Symbol, pool.Token1.Symbol)
			}
			fmt.Printf(" (%d hops)\n", len(path)-1)
		}
	}
}

// func main() {
// 	TestPathFindingDetails()
// }
