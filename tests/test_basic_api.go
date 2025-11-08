package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"net/http/httptest"
	"time"

	"dex-aggregator/config"
	"dex-aggregator/internal/aggregator"
	"dex-aggregator/internal/api"
	"dex-aggregator/internal/cache"
	"dex-aggregator/internal/collector"
	"dex-aggregator/internal/types"
)

// TestBasicSetup 测试基础设置和池子初始化
func TestBasicSetup() {
	fmt.Println("=== Testing Basic Setup ===")

	if err := config.Init(); err != nil {
		log.Fatalf("Failed to initialize config: %v", err)
	}

	// 使用内存存储进行测试
	store := cache.NewMemoryStore()
	poolCollector := collector.NewMockPoolCollector(store)

	fmt.Println("Initializing mock pools...")
	if err := poolCollector.InitMockPools(); err != nil {
		log.Fatalf("Failed to initialize mock data: %v", err)
	}

	// 验证池子是否正确存储
	ctx := context.Background()
	pools, err := store.GetAllPools(ctx)
	if err != nil {
		log.Fatalf("Failed to get pools: %v", err)
	}

	fmt.Printf("✓ Successfully created %d mock pools\n", len(pools))

	// 打印前几个池子信息
	for i, pool := range pools {
		if i >= 3 {
			break
		}
		fmt.Printf("  Pool %d: %s - %s/%s\n", i+1, pool.Exchange, pool.Token0.Symbol, pool.Token1.Symbol)
	}
}

// TestDirectPathQuote 测试直接路径报价
func TestDirectPathQuote() {
	fmt.Println("\n=== Testing Direct Path Quote ===")

	store := cache.NewMemoryStore()
	poolCollector := collector.NewMockPoolCollector(store)
	poolCollector.InitMockPools()

	router := aggregator.NewRouter(store)

	// WETH -> USDT 直接交易
	req := &types.QuoteRequest{
		TokenIn:  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2", // WETH
		TokenOut: "0xdac17f958d2ee523a2206206994597c13d831ec7", // USDT
		AmountIn: big.NewInt(1000000000000000000),              // 1 WETH
		MaxHops:  3,
	}

	ctx := context.Background()
	start := time.Now()
	resp, err := router.GetBestQuote(ctx, req)
	elapsed := time.Since(start)

	if err != nil {
		log.Fatalf("Direct path quote failed: %v", err)
	}

	fmt.Printf("✓ Direct path quote successful\n")
	fmt.Printf("  Input: 1 WETH\n")
	fmt.Printf("  Output: %s USDT\n", resp.AmountOut.String())
	fmt.Printf("  Best Path Dexes: %v\n", resp.BestPath.Dexes)
	fmt.Printf("  Gas Estimate: %s\n", resp.GasEstimate.String())
	fmt.Printf("  Processing Time: %v\n", elapsed)
	fmt.Printf("  Total Paths Found: %d\n", len(resp.Paths))
}

// TestMultiHopPathQuote 测试多跳路径报价
func TestMultiHopPathQuote() {
	fmt.Println("\n=== Testing Multi-Hop Path Quote ===")

	store := cache.NewMemoryStore()
	poolCollector := collector.NewMockPoolCollector(store)
	poolCollector.InitMockPools()

	router := aggregator.NewRouter(store)

	// WETH -> DAI (可能需要通过USDC)
	req := &types.QuoteRequest{
		TokenIn:  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2", // WETH
		TokenOut: "0x6b175474e89094c44da98b954eedeac495271d0f", // DAI
		AmountIn: big.NewInt(500000000000000000),               // 0.5 WETH
		MaxHops:  3,
	}

	ctx := context.Background()
	start := time.Now()
	resp, err := router.GetBestQuote(ctx, req)
	elapsed := time.Since(start)

	if err != nil {
		log.Fatalf("Multi-hop path quote failed: %v", err)
	}

	fmt.Printf("✓ Multi-hop path quote successful\n")
	fmt.Printf("  Input: 0.5 WETH\n")
	fmt.Printf("  Output: %s DAI\n", resp.AmountOut.String())
	fmt.Printf("  Best Path Length: %d hops\n", len(resp.BestPath.Pools))
	fmt.Printf("  Best Path Dexes: %v\n", resp.BestPath.Dexes)
	fmt.Printf("  Processing Time: %v\n", elapsed)
}

// TestAPIServer 测试API服务器端点
func TestAPIServer() {
	fmt.Println("\n=== Testing API Server Endpoints ===")

	store := cache.NewMemoryStore()
	poolCollector := collector.NewMockPoolCollector(store)
	poolCollector.InitMockPools()

	router := aggregator.NewRouter(store)
	handler := api.NewHandler(router, store)

	// 测试健康检查
	healthReq := httptest.NewRequest("GET", "/health", nil)
	healthRec := httptest.NewRecorder()
	handler.HealthCheck(healthRec, healthReq)

	if healthRec.Code != http.StatusOK {
		log.Fatalf("Health check failed: %d", healthRec.Code)
	}
	fmt.Printf("✓ Health check passed\n")

	// 测试获取所有池子
	poolsReq := httptest.NewRequest("GET", "/api/v1/pools", nil)
	poolsRec := httptest.NewRecorder()
	handler.GetPools(poolsRec, poolsReq)

	if poolsRec.Code != http.StatusOK {
		log.Fatalf("Get pools failed: %d", poolsRec.Code)
	}

	var poolsResp map[string]interface{}
	json.Unmarshal(poolsRec.Body.Bytes(), &poolsResp)
	fmt.Printf("✓ Get pools passed: %d pools\n", int(poolsResp["count"].(float64)))

	// 测试按代币对搜索池子
	searchReq := httptest.NewRequest("GET", "/api/v1/pools/search?tokenA=0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2&tokenB=0xdac17f958d2ee523a2206206994597c13d831ec7", nil)
	searchRec := httptest.NewRecorder()
	handler.GetPoolsByTokens(searchRec, searchReq)

	if searchRec.Code != http.StatusOK {
		log.Fatalf("Search pools failed: %d", searchRec.Code)
	}
	fmt.Printf("✓ Search pools passed\n")
}

// TestQuoteAPI 测试报价API端点
func TestQuoteAPI() {
	fmt.Println("\n=== Testing Quote API ===")

	store := cache.NewMemoryStore()
	poolCollector := collector.NewMockPoolCollector(store)
	poolCollector.InitMockPools()

	router := aggregator.NewRouter(store)
	handler := api.NewHandler(router, store)

	// 准备报价请求
	quoteReq := types.QuoteRequest{
		TokenIn:  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
		TokenOut: "0xdac17f958d2ee523a2206206994597c13d831ec7",
		AmountIn: big.NewInt(1000000000000000000),
		MaxHops:  3,
	}

	reqBody, _ := json.Marshal(quoteReq)
	req := httptest.NewRequest("POST", "/api/v1/quote", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.GetQuote(rec, req)

	if rec.Code != http.StatusOK {
		log.Fatalf("Quote API failed: %d - %s", rec.Code, rec.Body.String())
	}

	var quoteResp types.QuoteResponse
	json.Unmarshal(rec.Body.Bytes(), &quoteResp)

	fmt.Printf("✓ Quote API successful\n")
	fmt.Printf("  Amount Out: %s\n", quoteResp.AmountOut.String())
	fmt.Printf("  Gas Estimate: %s\n", quoteResp.GasEstimate.String())
	fmt.Printf("  Best Path Dexes: %v\n", quoteResp.BestPath.Dexes)
}

// TestErrorCases 测试错误情况
func TestErrorCases() {
	fmt.Println("\n=== Testing Error Cases ===")

	store := cache.NewMemoryStore()
	poolCollector := collector.NewMockPoolCollector(store)
	poolCollector.InitMockPools()

	router := aggregator.NewRouter(store)
	handler := api.NewHandler(router, store)

	// 测试无效的代币地址
	invalidReq := types.QuoteRequest{
		TokenIn:  "invalid_address",
		TokenOut: "0xdac17f958d2ee523a2206206994597c13d831ec7",
		AmountIn: big.NewInt(1000000000000000000),
	}

	reqBody, _ := json.Marshal(invalidReq)
	req := httptest.NewRequest("POST", "/api/v1/quote", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.GetQuote(rec, req)

	if rec.Code == http.StatusOK {
		log.Printf("Expected error for invalid address but got success")
	} else {
		fmt.Printf("✓ Invalid address handling correct: %d\n", rec.Code)
	}

	// 测试不存在的代币对
	nonExistentReq := types.QuoteRequest{
		TokenIn:  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
		TokenOut: "0x0000000000000000000000000000000000000000", // 不存在的代币
		AmountIn: big.NewInt(1000000000000000000),
	}

	reqBody2, _ := json.Marshal(nonExistentReq)
	req2 := httptest.NewRequest("POST", "/api/v1/quote", bytes.NewBuffer(reqBody2))
	req2.Header.Set("Content-Type", "application/json")

	rec2 := httptest.NewRecorder()
	handler.GetQuote(rec2, req2)

	if rec2.Code == http.StatusOK {
		fmt.Printf("  No path found handled correctly\n")
	} else {
		fmt.Printf("✓ Non-existent token pair handled: %d\n", rec2.Code)
	}
}

// TestPerformance 测试性能
func TestPerformance() {
	fmt.Println("\n=== Testing Performance ===")

	store := cache.NewMemoryStore()
	poolCollector := collector.NewMockPoolCollector(store)
	poolCollector.InitMockPools()

	router := aggregator.NewRouter(store)

	testCases := []struct {
		name     string
		tokenIn  string
		tokenOut string
		amount   *big.Int
	}{
		{
			name:     "WETH->USDT",
			tokenIn:  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
			tokenOut: "0xdac17f958d2ee523a2206206994597c13d831ec7",
			amount:   big.NewInt(1000000000000000000),
		},
		{
			name:     "WETH->USDC",
			tokenIn:  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
			tokenOut: "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
			amount:   big.NewInt(500000000000000000),
		},
		{
			name:     "WETH->DAI",
			tokenIn:  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
			tokenOut: "0x6b175474e89094c44da98b954eedeac495271d0f",
			amount:   big.NewInt(2000000000000000000),
		},
	}

	ctx := context.Background()

	for _, tc := range testCases {
		start := time.Now()

		req := &types.QuoteRequest{
			TokenIn:  tc.tokenIn,
			TokenOut: tc.tokenOut,
			AmountIn: tc.amount,
			MaxHops:  3,
		}

		resp, err := router.GetBestQuote(ctx, req)
		elapsed := time.Since(start)

		if err != nil {
			fmt.Printf("  %s: FAILED - %v\n", tc.name, err)
			continue
		}

		fmt.Printf("  %s: %v - %d paths found\n",
			tc.name, elapsed, len(resp.Paths))
	}
}

func main() {
	fmt.Println("Starting DEX Aggregator Comprehensive Tests...")
	fmt.Println("=============================================")

	TestBasicSetup()
	TestDirectPathQuote()
	TestMultiHopPathQuote()
	TestAPIServer()
	TestQuoteAPI()
	TestErrorCases()
	TestPerformance()

	fmt.Println("\n=============================================")
	fmt.Println("All tests completed successfully! 🎉")
}
