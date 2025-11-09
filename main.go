package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"dex-aggregator/config"
	"dex-aggregator/internal/aggregator"
	"dex-aggregator/internal/api"
	"dex-aggregator/internal/cache"
	"dex-aggregator/internal/collector"
	"dex-aggregator/internal/types" // 确保导入 types

	"github.com/gorilla/mux"
)

func main() {
	if err := config.Init(); err != nil {
		log.Fatalf("Failed to initialize config: %v", err)
	}

	log.Println("Starting DEX Aggregator with optimized configuration...")

	// Use two-level cache for better performance
	store := cache.NewTwoLevelCache(
		config.AppConfig.Redis.Addr,
		config.AppConfig.Redis.Password,
		config.AppConfig.Performance.CacheTTL,
	)

	// 修复: 将 []types.Exchange 转换为 []*types.Exchange
	exchangesPtrs := make([]*types.Exchange, len(config.AppConfig.DEX.Exchanges))
	for i := range config.AppConfig.DEX.Exchanges {
		exchangesPtrs[i] = &config.AppConfig.DEX.Exchanges[i]
	}

	poolCollector := collector.NewMockPoolCollector(store, exchangesPtrs)

	log.Println("Initializing mock pool data...")
	if err := poolCollector.InitMockPools(); err != nil {
		log.Fatalf("Failed to initialize mock data: %v", err)
	}

	router := aggregator.NewRouter(store, config.AppConfig.Performance)
	handler := api.NewHandler(router, store)

	r := mux.NewRouter()

	// API routes
	r.HandleFunc("/api/v1/quote", handler.GetQuote).Methods("POST")
	r.HandleFunc("/api/v1/pools", handler.GetPools).Methods("GET")
	r.HandleFunc("/api/v1/pools/search", handler.GetPoolsByTokens).Methods("GET")
	r.HandleFunc("/api/v1/pools/{address}", handler.GetPoolByAddress).Methods("GET")
	r.HandleFunc("/health", handler.HealthCheck).Methods("GET")
	r.HandleFunc("/config", handler.GetConfig).Methods("GET")
	r.HandleFunc("/cache/stats", handler.GetCacheStats).Methods("GET")

	// Root endpoint with system information
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `
        <html>
            <head><title>DEX Aggregator - Optimized</title></head>
            <body>
                <h1>DEX Aggregator Service (Optimized)</h1>
                <p>Performance-optimized version with two-level caching</p>
                <ul>
                    <li>Server Port: %s</li>
                    <li>Redis: %s</li>
                    <li>Base Tokens: %d configured</li>
                    <li>Max Concurrent Paths: %d</li>
                    <li>Max Slippage: %.2f%%</li>
                </ul>
                <p>Available endpoints:</p>
                <ul>
                    <li><a href="/api/v1/pools">GET /api/v1/pools</a> - Get all pools</li>
                    <li><a href="/api/v1/pools/search?tokenA=0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2&tokenB=0xdAC17F958D2ee523a2206206994597C13D831ec7">GET /api/v1/pools/search</a> - Search pools</li>
                    <li><a href="/config">GET /config</a> - View current configuration</li>
                    <li><a href="/cache/stats">GET /cache/stats</a> - Cache performance</li>
                    <li>POST /api/v1/quote - Quote endpoint</li>
                    <li><a href="/health">GET /health</a> - Health check</li>
                </ul>
            </body>
        </html>
        `, config.AppConfig.Server.Port, config.AppConfig.Redis.Addr,
			// 更改: BaseTokens 现在在顶层
			len(config.AppConfig.BaseTokens),
			config.AppConfig.Performance.MaxConcurrentPaths,
			config.AppConfig.Performance.MaxSlippage)
	})

	port := ":" + config.AppConfig.Server.Port
	log.Printf("HTTP server starting on http://localhost%s", port)
	log.Printf("Performance settings: %d max concurrent paths, %.2f%% max slippage",
		config.AppConfig.Performance.MaxConcurrentPaths,
		config.AppConfig.Performance.MaxSlippage)

	server := &http.Server{
		Addr:         port,
		Handler:      r,
		ReadTimeout:  time.Duration(config.AppConfig.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(config.AppConfig.Server.WriteTimeout) * time.Second,
	}

	log.Fatal(server.ListenAndServe())
}
