package main

import (
	"dex-aggregator/config"
	"dex-aggregator/internal/aggregator"
	"dex-aggregator/internal/cache"
	"dex-aggregator/internal/collector"
	"fmt"
	"log"
)

// TestIntegration 运行完整的集成测试
func TestIntegration() {
	fmt.Println("=== Running Integration Tests ===")

	// 初始化配置
	if err := config.Init(); err != nil {
		log.Fatalf("Failed to initialize config: %v", err)
	}

	// 测试两级缓存
	store := cache.NewTwoLevelCache(
		config.AppConfig.Redis.Addr,
		config.AppConfig.Redis.Password,
		config.AppConfig.Performance.CacheTTL,
	)

	// 初始化模拟数据
	collector := collector.NewMockPoolCollector(store)
	if err := collector.InitMockPools(); err != nil {
		log.Fatalf("Failed to initialize mock pools: %v", err)
	}

	// 测试路由
	router := aggregator.NewRouter(store)

	fmt.Println("✓ Integration test setup completed")
	fmt.Printf("  Using two-level cache with %v TTL\n", config.AppConfig.Performance.CacheTTL)
	fmt.Printf("  Max concurrent paths: %d\n", config.AppConfig.Performance.MaxConcurrentPaths)

	// 这里可以添加更多的集成测试逻辑
}

func main() {
	TestIntegration()
}
