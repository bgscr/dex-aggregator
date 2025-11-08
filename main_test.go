package main

import (
	"context"
	"dex-aggregator/config"
	"dex-aggregator/internal/aggregator"
	"dex-aggregator/internal/cache"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type MainTestSuite struct {
	suite.Suite
	router *aggregator.Router
	cache  cache.Store
	ctx    context.Context
}

func (suite *MainTestSuite) SetupTest() {
	// 初始化配置
	err := config.Init()
	suite.NoError(err)

	// 创建缓存
	suite.cache = cache.NewTwoLevelCache(
		config.AppConfig.Redis.Addr,
		config.AppConfig.Redis.Password,
		config.AppConfig.Performance.CacheTTL,
	)

	perfConfig := config.PerformanceConfig{MaxSlippage: 5.0, MaxHops: 3, MaxConcurrentPaths: 10}
	suite.router = aggregator.NewRouter(suite.cache, perfConfig)
	suite.ctx = context.Background()
}

func (suite *MainTestSuite) TestConfigInitialization() {
	assert.NotNil(suite.T(), config.AppConfig)
	assert.Equal(suite.T(), "8080", config.AppConfig.Server.Port)
	assert.Greater(suite.T(), len(config.AppConfig.DEX.BaseTokens), 0)
}

func (suite *MainTestSuite) TestRouterCreation() {
	assert.NotNil(suite.T(), suite.router)
}

func TestMainSuite(t *testing.T) {
	suite.Run(t, new(MainTestSuite))
}
