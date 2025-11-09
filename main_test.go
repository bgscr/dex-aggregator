package main

import (
	"log"
	"os"
	"testing"
	"time"

	"dex-aggregator/config"

	"github.com/stretchr/testify/assert"
)

// This test suite focuses on the main package, particularly configuration loading
// and basic application setup checks.

// setup prepares the environment for tests
func setup(t *testing.T) {
	// Set predictable environment variables for testing
	os.Setenv("SERVER_PORT", "8080")
	os.Setenv("REDIS_ADDR", "localhost:6379")
	os.Setenv("BASE_TOKENS", "0xtokenA,0xtokenB,0xtokenC,0xtokenD")
	os.Setenv("MAX_CONCURRENT_PATHS", "5")
	os.Setenv("MAX_SLIPPAGE", "2.5")
	os.Setenv("CACHE_TTL_SECONDS", "60")

	// Set a path for the YAML config (even if it doesn't exist for this test)
	// This mimics the Init logic
	os.Chdir("..") // Move up to project root to find config/config.yaml
}

// teardown cleans up environment variables
func teardown(t *testing.T) {
	os.Unsetenv("SERVER_PORT")
	os.Unsetenv("REDIS_ADDR")
	os.Unsetenv("BASE_TOKENS")
	os.Unsetenv("MAX_CONCURRENT_PATHS")
	os.Unsetenv("MAX_SLIPPAGE")
	os.Unsetenv("CACHE_TTL_SECONDS")
	os.Chdir("dex-aggregator") // Go back into the package directory
}

func TestConfigInitialization_WithEnvVars(t *testing.T) {
	// Note: Can't run setup/teardown with Chdir in parallel tests
	setup(t)
	defer teardown(t)

	log.Println("Current working directory:", os.Getenv("PWD"))

	if err := config.Init(); err != nil {
		t.Fatalf("Failed to initialize config: %v", err)
	}

	assert.NotNil(t, config.AppConfig)

	// Check values loaded from environment
	assert.Equal(t, "8080", config.AppConfig.Server.Port)
	assert.Equal(t, "localhost:6379", config.AppConfig.Redis.Addr)
	assert.Equal(t, 5, config.AppConfig.Performance.MaxConcurrentPaths)
	assert.Equal(t, 2.5, config.AppConfig.Performance.MaxSlippage)
	assert.Equal(t, 60*time.Second, config.AppConfig.Performance.CacheTTL)

	// 修复: 检查 config.AppConfig.BaseTokens (顶层)
	assert.Equal(t, 4, len(config.AppConfig.BaseTokens))
	assert.Equal(t, "0xtokenA", config.AppConfig.BaseTokens[0])
}

func TestConfigInitialization_Defaults(t *testing.T) {
	// Clear all env vars to test defaults
	os.Clearenv()

	// We still need to be in the root to find the YAML, even if it's just for defaults
	os.Chdir("..")
	defer os.Chdir("dex-aggregator")

	if err := config.Init(); err != nil {
		t.Fatalf("Failed to initialize config: %v", err)
	}

	assert.NotNil(t, config.AppConfig)

	// Check default values (assuming config.yaml might not be found or might have defaults)
	// These checks depend on what's in config.yaml or the hardcoded fallbacks
	assert.Contains(t, []string{"8080", "8081"}, config.AppConfig.Server.Port) // 8080 is default in YAML and fallback
	assert.Equal(t, "localhost:6379", config.AppConfig.Redis.Addr)
	assert.Equal(t, 10, config.AppConfig.Performance.MaxConcurrentPaths) // Default fallback
	assert.Equal(t, 5.0, config.AppConfig.Performance.MaxSlippage)       // Default fallback
	assert.Equal(t, 300*time.Second, config.AppConfig.Performance.CacheTTL)

	// Check default base tokens (fallback)
	assert.Equal(t, 4, len(config.AppConfig.BaseTokens))
	assert.Equal(t, "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2", config.AppConfig.BaseTokens[0])
}
