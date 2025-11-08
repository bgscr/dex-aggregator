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

	"github.com/gorilla/mux"
)

func main() {
	// 初始化配置
	if err := config.Init(); err != nil {
		log.Fatalf("Failed to initialize config: %v", err)
	}

	log.Println("Starting DEX Aggregator with Redis and environment config...")

	// 使用配置初始化Redis缓存
	store := cache.NewRedisStore(
		config.AppConfig.Redis.Addr,
		config.AppConfig.Redis.Password,
	)

	// 初始化数据收集器
	poolCollector := collector.NewMockPoolCollector(store)

	// 初始化模拟数据
	log.Println("Initializing mock pool data...")
	if err := poolCollector.InitMockPools(); err != nil {
		log.Fatalf("Failed to initialize mock data: %v", err)
	}

	// 使用配置中的基础代币初始化路由引擎
	router := aggregator.NewRouter(store, config.AppConfig.DEX.BaseTokens)

	// 初始化API handler
	handler := api.NewHandler(router, store)

	// 设置HTTP路由
	r := mux.NewRouter()

	// 注册API端点
	r.HandleFunc("/api/v1/quote", handler.GetQuote).Methods("POST")
	r.HandleFunc("/api/v1/pools", handler.GetPools).Methods("GET")
	r.HandleFunc("/api/v1/pools/search", handler.GetPoolsByTokens).Methods("GET")
	r.HandleFunc("/health", handler.HealthCheck).Methods("GET")
	r.HandleFunc("/config", handler.GetConfig).Methods("GET") // 新增配置查看端点

	// 主页
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `
        <html>
            <head><title>DEX Aggregator</title></head>
            <body>
                <h1>DEX Aggregator Service</h1>
                <p>Configuration: Using environment variables</p>
                <ul>
                    <li>Server Port: %s</li>
                    <li>Redis: %s</li>
                    <li>Base Tokens: %d configured</li>
                </ul>
                <p>Available endpoints:</p>
                <ul>
                    <li><a href="/api/v1/pools">GET /api/v1/pools</a> - Get all pools</li>
                    <li><a href="/api/v1/pools/search?tokenA=0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2&tokenB=0xdAC17F958D2ee523a2206206994597C13D831ec7">GET /api/v1/pools/search</a> - Search pools</li>
                    <li><a href="/config">GET /config</a> - View current configuration</li>
                    <li>POST /api/v1/quote - Quote endpoint</li>
                    <li><a href="/health">GET /health</a> - Health check</li>
                </ul>
            </body>
        </html>
        `, config.AppConfig.Server.Port, config.AppConfig.Redis.Addr, len(config.AppConfig.DEX.BaseTokens))
	})

	// 启动HTTP服务器
	port := ":" + config.AppConfig.Server.Port
	log.Printf("HTTP server starting on http://localhost%s", port)

	server := &http.Server{
		Addr:         port,
		Handler:      r,
		ReadTimeout:  time.Duration(config.AppConfig.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(config.AppConfig.Server.WriteTimeout) * time.Second,
	}

	log.Fatal(server.ListenAndServe())
}
