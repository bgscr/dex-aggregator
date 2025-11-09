package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"dex-aggregator/internal/types" // 新增: 导入 types

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3" // 新增: 导入 yaml
)

type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Redis       RedisConfig       `yaml:"redis"`
	Ethereum    EthereumConfig    `yaml:"ethereum"`
	DEX         DEXConfig         `yaml:"dex"`
	BaseTokens  []string          `yaml:"base_tokens"` // 从 DEXConfig 移到这里
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
	// BaseTokens 字段被移到顶层 Config 结构体
	Exchanges []types.Exchange `yaml:"exchanges"` // 新增
}

type PerformanceConfig struct {
	MaxConcurrentPaths   int           `json:"max_concurrent_paths" yaml:"max_concurrent_paths"` // 注意: api/handler.go 中 GetConfig 依然使用 json 标签, 暂时保留
	CacheTTL             time.Duration `json:"cache_ttl" yaml:"cache_ttl_seconds"`
	RequestTimeout       time.Duration `json:"request_timeout" yaml:"request_timeout_seconds"`
	MaxHops              int           `json:"max_hops" yaml:"max_hops"`
	MaxSlippage          float64       `json:"max_slippage" yaml:"max_slippage"`
	MaxPaths             int           `json:"max_paths" yaml:"max_paths"`
	GraphRefreshInterval time.Duration `json:"graph_refresh_interval" yaml:"graph_refresh_seconds"`
}

var AppConfig *Config

// loadConfigFromFile 从 YAML 文件加载默认配置
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
	// 1. 初始化一个空配置
	AppConfig = &Config{}

	// 2. 从 YAML 加载默认值
	// 注意：这里的路径是相对于项目根目录
	if err := loadConfigFromFile("config/config.yaml", AppConfig); err != nil {
		log.Printf("Warning: Failed to load config.yaml: %v. Using defaults.", err)
	}

	// 3. 加载 .env 文件, 这会把 .env 里的值设置到环境变量, 优先于系统环境变量
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found, using environment variables")
	}

	// 4. 使用环境变量覆盖 YAML 的值
	// 如果环境变量未设置, getEnv 将使用 AppConfig 中已有的值 (来自YAML) 作为默认值
	AppConfig.Server.Port = getEnv("SERVER_PORT", AppConfig.Server.Port, "8080")
	AppConfig.Server.ReadTimeout = getEnvAsInt("SERVER_READ_TIMEOUT", AppConfig.Server.ReadTimeout, 15)
	AppConfig.Server.WriteTimeout = getEnvAsInt("SERVER_WRITE_TIMEOUT", AppConfig.Server.WriteTimeout, 15)

	AppConfig.Redis.Addr = getEnv("REDIS_ADDR", AppConfig.Redis.Addr, "localhost:6379")
	AppConfig.Redis.Password = getEnv("REDIS_PASSWORD", AppConfig.Redis.Password, "")
	AppConfig.Redis.DB = getEnvAsInt("REDIS_DB", AppConfig.Redis.DB, 0)

	AppConfig.Ethereum.RPCURL = getEnv("ETH_RPC_URL", AppConfig.Ethereum.RPCURL, "wss://mainnet.infura.io/ws/v3/YOUR-PROJECT-ID")
	AppConfig.Ethereum.ChainID = getEnvAsInt64("ETH_CHAIN_ID", AppConfig.Ethereum.ChainID, 1)

	// 如果 YAML 中没有 base_tokens, 则使用这里的硬编码默认值
	defaultBaseTokens := []string{
		"0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
		"0xdac17f958d2ee523a2206206994597c13d831ec7",
		"0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"0x6b175474e89094c44da98b954eedeac495271d0f",
	}
	AppConfig.BaseTokens = getEnvAsSlice("BASE_TOKENS", ",", AppConfig.BaseTokens, defaultBaseTokens)

	// Performance (注意: YAML 和 Env 变量的 key 可能不同)
	AppConfig.Performance.MaxConcurrentPaths = getEnvAsInt("MAX_CONCURRENT_PATHS", AppConfig.Performance.MaxConcurrentPaths, 10)
	AppConfig.Performance.CacheTTL = time.Duration(getEnvAsInt("CACHE_TTL_SECONDS", int(AppConfig.Performance.CacheTTL.Seconds()), 300)) * time.Second
	AppConfig.Performance.RequestTimeout = time.Duration(getEnvAsInt("REQUEST_TIMEOUT_SECONDS", int(AppConfig.Performance.RequestTimeout.Seconds()), 30)) * time.Second
	AppConfig.Performance.MaxHops = getEnvAsInt("MAX_HOPS", AppConfig.Performance.MaxHops, 3)
	AppConfig.Performance.MaxSlippage = getEnvAsFloat("MAX_SLIPPAGE", AppConfig.Performance.MaxSlippage, 5.0)
	AppConfig.Performance.MaxPaths = getEnvAsInt("MAX_PATHS", AppConfig.Performance.MaxPaths, 20)
	AppConfig.Performance.GraphRefreshInterval = time.Duration(getEnvAsInt("GRAPH_REFRESH_SECONDS", int(AppConfig.Performance.GraphRefreshInterval.Seconds()), 30)) * time.Second

	return nil
}

// getEnv (重载) - 如果 envValue 未设置, 使用 yamlValue, 如果 yamlValue 也未设置, 使用 fallback
func getEnv(key string, yamlValue string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value // 环境变量优先级最高
	}
	if yamlValue != "" {
		return yamlValue // YAML 文件次之
	}
	return fallback // 默认值
}

// getEnvAsInt (重载)
func getEnvAsInt(key string, yamlValue int, fallback int) int {
	valueStr := os.Getenv(key)
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value // 环境变量
	}
	if yamlValue != 0 { // 假设 0 不是一个有效的YAML值 (或者根据需要调整)
		return yamlValue // YAML
	}
	return fallback // 默认
}

// getEnvAsInt64 (重载)
func getEnvAsInt64(key string, yamlValue int64, fallback int64) int64 {
	valueStr := os.Getenv(key)
	if value, err := strconv.ParseInt(valueStr, 10, 64); err == nil {
		return value // 环境变量
	}
	if yamlValue != 0 {
		return yamlValue // YAML
	}
	return fallback // 默认
}

// getEnvAsFloat (重载)
func getEnvAsFloat(key string, yamlValue float64, fallback float64) float64 {
	valueStr := os.Getenv(key)
	if value, err := strconv.ParseFloat(valueStr, 64); err == nil {
		return value // 环境变量
	}
	if yamlValue != 0.0 {
		return yamlValue // YAML
	}
	return fallback // 默认
}

// getEnvAsSlice (重载)
func getEnvAsSlice(key, separator string, yamlValue []string, fallback []string) []string {
	valueStr := os.Getenv(key)
	if valueStr != "" {
		return strings.Split(valueStr, separator) // 环境变量
	}
	if len(yamlValue) > 0 {
		return yamlValue // YAML
	}
	return fallback // 默认
}
