package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"dex-aggregator/internal/types"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Redis       RedisConfig       `yaml:"redis"`
	Ethereum    EthereumConfig    `yaml:"ethereum"`
	DEX         DEXConfig         `yaml:"dex"`
	BaseTokens  []string          `yaml:"base_tokens"`
	Performance PerformanceConfig `yaml:"performance"`
}

type ServerConfig struct {
	Port         string `yaml:"port"`
	ReadTimeout  int    `yaml:"read_timeout"`
	WriteTimeout int    `yaml:"write_timeout"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type EthereumConfig struct {
	RPCURL  string `yaml:"rpc_url"`
	ChainID int64  `yaml:"chain_id"`
}

type DEXConfig struct {
	Exchanges []types.Exchange `yaml:"exchanges"`
}

type PerformanceConfig struct {
	MaxConcurrentPaths   int           `json:"max_concurrent_paths" yaml:"max_concurrent_paths"`
	CacheTTL             time.Duration `json:"cache_ttl" yaml:"cache_ttl_seconds"`
	RequestTimeout       time.Duration `json:"request_timeout" yaml:"request_timeout_seconds"`
	MaxHops              int           `json:"max_hops" yaml:"max_hops"`
	MaxSlippage          float64       `json:"max_slippage" yaml:"max_slippage"`
	MaxPaths             int           `json:"max_paths" yaml:"max_paths"`
	GraphRefreshInterval time.Duration `json:"graph_refresh_interval" yaml:"graph_refresh_seconds"`
}

var AppConfig *Config

// loadConfigFromFile loads default configuration from a YAML file.
func loadConfigFromFile(path string, config *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Warning: YAML config file not found at %s. Using env vars and defaults only.", path)
			return nil
		}
		return err
	}
	if err = yaml.Unmarshal(data, config); err != nil {
		return err
	}
	log.Printf("Loaded configuration defaults from %s", path)
	return nil
}

func Init() error {
	AppConfig = &Config{}

	if err := loadConfigFromFile("config/config.yaml", AppConfig); err != nil {
		log.Printf("Warning: Failed to load config.yaml: %v. Using defaults.", err)
	}

	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found, using environment variables")
	}

	AppConfig.Server.Port = getEnv("SERVER_PORT", AppConfig.Server.Port, "8080")
	AppConfig.Server.ReadTimeout = getEnvAsInt("SERVER_READ_TIMEOUT", AppConfig.Server.ReadTimeout, 15)
	AppConfig.Server.WriteTimeout = getEnvAsInt("SERVER_WRITE_TIMEOUT", AppConfig.Server.WriteTimeout, 15)

	AppConfig.Redis.Addr = getEnv("REDIS_ADDR", AppConfig.Redis.Addr, "localhost:6379")
	AppConfig.Redis.Password = getEnv("REDIS_PASSWORD", AppConfig.Redis.Password, "")
	AppConfig.Redis.DB = getEnvAsInt("REDIS_DB", AppConfig.Redis.DB, 0)

	AppConfig.Ethereum.RPCURL = getEnv("ETH_RPC_URL", AppConfig.Ethereum.RPCURL, "wss://mainnet.infura.io/ws/v3/YOUR-PROJECT-ID")
	AppConfig.Ethereum.ChainID = getEnvAsInt64("ETH_CHAIN_ID", AppConfig.Ethereum.ChainID, 1)

	defaultBaseTokens := []string{
		"0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
		"0xdac17f958d2ee523a2206206994597c13d831ec7",
		"0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"0x6b175474e89094c44da98b954eedeac495271d0f",
	}
	AppConfig.BaseTokens = getEnvAsSlice("BASE_TOKENS", ",", AppConfig.BaseTokens, defaultBaseTokens)

	AppConfig.Performance.MaxConcurrentPaths = getEnvAsInt("MAX_CONCURRENT_PATHS", AppConfig.Performance.MaxConcurrentPaths, 10)
	AppConfig.Performance.CacheTTL = time.Duration(getEnvAsInt("CACHE_TTL_SECONDS", int(AppConfig.Performance.CacheTTL.Seconds()), 300)) * time.Second
	AppConfig.Performance.RequestTimeout = time.Duration(getEnvAsInt("REQUEST_TIMEOUT_SECONDS", int(AppConfig.Performance.RequestTimeout.Seconds()), 30)) * time.Second
	AppConfig.Performance.MaxHops = getEnvAsInt("MAX_HOPS", AppConfig.Performance.MaxHops, 3)
	AppConfig.Performance.MaxSlippage = getEnvAsFloat("MAX_SLIPPAGE", AppConfig.Performance.MaxSlippage, 5.0)
	AppConfig.Performance.MaxPaths = getEnvAsInt("MAX_PATHS", AppConfig.Performance.MaxPaths, 20)
	AppConfig.Performance.GraphRefreshInterval = time.Duration(getEnvAsInt("GRAPH_REFRESH_SECONDS", int(AppConfig.Performance.GraphRefreshInterval.Seconds()), 30)) * time.Second

	return nil
}

// getEnv returns env value if set, otherwise yamlValue if not empty, otherwise fallback.
func getEnv(key string, yamlValue string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	if yamlValue != "" {
		return yamlValue
	}
	return fallback
}

// getEnvAsInt returns env int if set, otherwise yamlValue if non-zero, otherwise fallback.
func getEnvAsInt(key string, yamlValue int, fallback int) int {
	valueStr := os.Getenv(key)
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	if yamlValue != 0 {
		return yamlValue
	}
	return fallback
}

// getEnvAsInt64 returns env int64 if set, otherwise yamlValue if non-zero, otherwise fallback.
func getEnvAsInt64(key string, yamlValue int64, fallback int64) int64 {
	valueStr := os.Getenv(key)
	if value, err := strconv.ParseInt(valueStr, 10, 64); err == nil {
		return value
	}
	if yamlValue != 0 {
		return yamlValue
	}
	return fallback
}

// getEnvAsFloat returns env float64 if set, otherwise yamlValue if non-zero, otherwise fallback.
func getEnvAsFloat(key string, yamlValue float64, fallback float64) float64 {
	valueStr := os.Getenv(key)
	if value, err := strconv.ParseFloat(valueStr, 64); err == nil {
		return value
	}
	if yamlValue != 0.0 {
		return yamlValue
	}
	return fallback
}

// getEnvAsSlice returns env slice if set, otherwise yamlValue if non-empty, otherwise fallback.
func getEnvAsSlice(key, separator string, yamlValue []string, fallback []string) []string {
	valueStr := os.Getenv(key)
	if valueStr != "" {
		return strings.Split(valueStr, separator)
	}
	if len(yamlValue) > 0 {
		return yamlValue
	}
	return fallback
}
