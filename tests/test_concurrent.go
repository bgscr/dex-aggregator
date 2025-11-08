package main

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"dex-aggregator/internal/aggregator"
	"dex-aggregator/internal/cache"
	"dex-aggregator/internal/collector"
	"dex-aggregator/internal/types"
)

func TestConcurrentQuotes() {
	fmt.Println("=== Testing Concurrent Quotes ===")

	store := cache.NewMemoryStore()
	poolCollector := collector.NewMockPoolCollector(store)
	poolCollector.InitMockPools()

	router := aggregator.NewRouter(store)
	ctx := context.Background()

	// 定义多个并发请求
	requests := []types.QuoteRequest{
		{
			TokenIn:  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
			TokenOut: "0xdac17f958d2ee523a2206206994597c13d831ec7",
			AmountIn: big.NewInt(1000000000000000000),
			MaxHops:  3,
		},
		{
			TokenIn:  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
			TokenOut: "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
			AmountIn: big.NewInt(2000000000000000000),
			MaxHops:  3,
		},
		{
			TokenIn:  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
			TokenOut: "0x6b175474e89094c44da98b954eedeac495271d0f",
			AmountIn: big.NewInt(500000000000000000),
			MaxHops:  3,
		},
		{
			TokenIn:  "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
			TokenOut: "0xdac17f958d2ee523a2206206994597c13d831ec7",
			AmountIn: big.NewInt(1000000000), // 1000 USDC
			MaxHops:  3,
		},
	}

	var wg sync.WaitGroup
	results := make(chan string, len(requests))
	errors := make(chan error, len(requests))

	start := time.Now()

	// 并发执行所有请求
	for i, req := range requests {
		wg.Add(1)
		go func(idx int, request types.QuoteRequest) {
			defer wg.Done()

			reqStart := time.Now()
			resp, err := router.GetBestQuote(ctx, &request)
			reqElapsed := time.Since(reqStart)

			if err != nil {
				errors <- fmt.Errorf("Request %d failed: %v", idx, err)
				return
			}

			results <- fmt.Sprintf("Req %d: %s → %s = %s (in %v, %d paths)",
				idx,
				getTokenSymbol(request.TokenIn),
				getTokenSymbol(request.TokenOut),
				resp.AmountOut.String(),
				reqElapsed,
				len(resp.Paths))
		}(i, req)
	}

	// 等待所有请求完成
	wg.Wait()
	close(results)
	close(errors)

	totalElapsed := time.Since(start)

	// 打印结果
	fmt.Printf("Concurrent test completed in %v\n", totalElapsed)
	fmt.Printf("Results:\n")

	for result := range results {
		fmt.Printf("  ✓ %s\n", result)
	}

	// 打印错误
	for err := range errors {
		fmt.Printf("  ✗ %s\n", err)
	}
}

func getTokenSymbol(address string) string {
	symbols := map[string]string{
		"0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2": "WETH",
		"0xdac17f958d2ee523a2206206994597c13d831ec7": "USDT",
		"0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48": "USDC",
		"0x6b175474e89094c44da98b954eedeac495271d0f": "DAI",
	}

	if symbol, exists := symbols[address]; exists {
		return symbol
	}
	return "UNKNOWN"
}

func main() {
	TestConcurrentQuotes()
}
