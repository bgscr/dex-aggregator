package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Server   ServerConfig
	Redis    RedisConfig
	Ethereum EthereumConfig
	DEX      DEXConfig
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

var AppConfig *Config

func Init() error {
	// 加载 .env 文件
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found, using default environment variables")
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
				"0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2", // WETH
				"0xdac17f958d2ee523a2206206994597c13d831ec7", // USDT
				"0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48", // USDC
				"0x6b175474e89094c44da98b954eedeac495271d0f", // DAI
			}),
		},
	}

	return nil
}

// 辅助函数：获取环境变量，如果不存在则返回默认值
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

func getEnvAsSlice(key, separator string, defaultValue []string) []string {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}
	return strings.Split(valueStr, separator)
}
