package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Server      ServerConfig
	Redis       RedisConfig
	Ethereum    EthereumConfig
	DEX         DEXConfig
	Performance PerformanceConfig
}

type ServerConfig struct {
	Port         string
	ReadTimeout  int
	WriteTimeout int
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type EthereumConfig struct {
	RPCURL  string
	ChainID int64
}

type DEXConfig struct {
	BaseTokens []string
}

type PerformanceConfig struct {
	MaxConcurrentPaths int           `json:"max_concurrent_paths"`
	CacheTTL           time.Duration `json:"cache_ttl"`
	RequestTimeout     time.Duration `json:"request_timeout"`
	MaxHops            int           `json:"max_hops"`
	MaxSlippage        float64       `json:"max_slippage"`
	MaxPaths           int           `json:"max_paths"`
}

var AppConfig *Config

func Init() error {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found, using environment variables")
	}

	AppConfig = &Config{
		Server: ServerConfig{
			Port:         getEnv("SERVER_PORT", "8080"),
			ReadTimeout:  getEnvAsInt("SERVER_READ_TIMEOUT", 15),
			WriteTimeout: getEnvAsInt("SERVER_WRITE_TIMEOUT", 15),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
		},
		Ethereum: EthereumConfig{
			RPCURL:  getEnv("ETH_RPC_URL", "wss://mainnet.infura.io/ws/v3/YOUR-PROJECT-ID"),
			ChainID: getEnvAsInt64("ETH_CHAIN_ID", 1),
		},
		DEX: DEXConfig{
			BaseTokens: getEnvAsSlice("BASE_TOKENS", ",", []string{
				"0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
				"0xdac17f958d2ee523a2206206994597c13d831ec7",
				"0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
				"0x6b175474e89094c44da98b954eedeac495271d0f",
			}),
		},
		Performance: PerformanceConfig{
			MaxConcurrentPaths: getEnvAsInt("MAX_CONCURRENT_PATHS", 10),
			CacheTTL:           time.Duration(getEnvAsInt("CACHE_TTL_SECONDS", 300)) * time.Second,
			RequestTimeout:     time.Duration(getEnvAsInt("REQUEST_TIMEOUT_SECONDS", 30)) * time.Second,
			MaxHops:            getEnvAsInt("MAX_HOPS", 3),
			MaxSlippage:        getEnvAsFloat("MAX_SLIPPAGE", 5.0),
			MaxPaths:           getEnvAsInt("MAX_PATHS", 20),
		},
	}

	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}

func getEnvAsInt64(key string, defaultValue int64) int64 {
	valueStr := getEnv(key, "")
	if value, err := strconv.ParseInt(valueStr, 10, 64); err == nil {
		return value
	}
	return defaultValue
}

func getEnvAsFloat(key string, defaultValue float64) float64 {
	valueStr := getEnv(key, "")
	if value, err := strconv.ParseFloat(valueStr, 64); err == nil {
		return value
	}
	return defaultValue
}

func getEnvAsSlice(key, separator string, defaultValue []string) []string {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}
	return strings.Split(valueStr, separator)
}
