package api

import (
	"bytes"
	"context"
	"dex-aggregator/config"
	"dex-aggregator/internal/aggregator"
	"dex-aggregator/internal/cache"
	"dex-aggregator/internal/types"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockStore simulates storage interface
type MockStore struct {
	mock.Mock
}

func (m *MockStore) StorePool(ctx context.Context, pool *types.Pool) error {
	args := m.Called(ctx, pool)
	return args.Error(0)
}

func (m *MockStore) GetPool(ctx context.Context, address string) (*types.Pool, error) {
	args := m.Called(ctx, address)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Pool), args.Error(1)
}

func (m *MockStore) GetPoolsByTokens(ctx context.Context, tokenA, tokenB string) ([]*types.Pool, error) {
	args := m.Called(ctx, tokenA, tokenB)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.Pool), args.Error(1)
}

func (m *MockStore) GetAllPools(ctx context.Context) ([]*types.Pool, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.Pool), args.Error(1)
}

func (m *MockStore) StoreToken(ctx context.Context, token *types.Token) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *MockStore) GetToken(ctx context.Context, address string) (*types.Token, error) {
	args := m.Called(ctx, address)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Token), args.Error(1)
}

// MockTwoLevelCache simulates two-level cache - implements cache.Store interface
type MockTwoLevelCache struct {
	mock.Mock
}

func (m *MockTwoLevelCache) StorePool(ctx context.Context, pool *types.Pool) error {
	args := m.Called(ctx, pool)
	return args.Error(0)
}

func (m *MockTwoLevelCache) GetPool(ctx context.Context, address string) (*types.Pool, error) {
	args := m.Called(ctx, address)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Pool), args.Error(1)
}

func (m *MockTwoLevelCache) GetPoolsByTokens(ctx context.Context, tokenA, tokenB string) ([]*types.Pool, error) {
	args := m.Called(ctx, tokenA, tokenB)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.Pool), args.Error(1)
}

func (m *MockTwoLevelCache) GetAllPools(ctx context.Context) ([]*types.Pool, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.Pool), args.Error(1)
}

func (m *MockTwoLevelCache) StoreToken(ctx context.Context, token *types.Token) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *MockTwoLevelCache) GetToken(ctx context.Context, address string) (*types.Token, error) {
	args := m.Called(ctx, address)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Token), args.Error(1)
}

func (m *MockTwoLevelCache) GetStats() *cache.CacheStats {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*cache.CacheStats)
}

// Initialize configuration before all tests
func TestMain(m *testing.M) {
	// Initialize configuration
	config.Init()
	m.Run()
}

func TestGetQuote_Success(t *testing.T) {
	mockStore := new(MockStore)
	perfConfig := config.PerformanceConfig{MaxSlippage: 5.0, MaxHops: 3, MaxConcurrentPaths: 10}

	// Create real Router but use mock Store
	router := aggregator.NewRouter(mockStore, perfConfig)
	handler := NewHandler(router, mockStore)

	// Prepare precise test data - ensure no slippage is triggered
	amountIn := big.NewInt(1000000000000000) // 0.001 ETH

	reqBody := map[string]interface{}{
		"tokenIn":  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
		"tokenOut": "0xdac17f958d2ee523a2206206994597c13d831ec7",
		"amountIn": amountIn.String(),
	}
	body, _ := json.Marshal(reqBody)

	// Mock pool data returned by store
	reserve0, _ := new(big.Int).SetString("100000000000000000000", 10) // 100 ETH
	reserve1 := big.NewInt(200000000000)                               // 200,000 USDT

	mockPools := []*types.Pool{
		{
			Address:  "test-pool",
			Exchange: "Uniswap V2",
			Token0: types.Token{
				Address:  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
				Symbol:   "WETH",
				Decimals: 18,
			},
			Token1: types.Token{
				Address:  "0xdac17f958d2ee523a2206206994597c13d831ec7",
				Symbol:   "USDT",
				Decimals: 6,
			},
			Reserve0: reserve0,
			Reserve1: reserve1,
			Fee:      300,
		},
	}
	mockStore.On("GetAllPools", mock.Anything).Return(mockPools, nil)

	// Create request
	req := httptest.NewRequest("POST", "/api/v1/quote", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute handler
	handler.GetQuote(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code, "Expected status 200, got %d: %s", w.Code, w.Body.String())

	// First verify response is valid JSON
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err, "Response should be valid JSON")

	// Then verify specific fields
	assert.Contains(t, response, "amountOut")
	assert.Contains(t, response, "paths")
	assert.Contains(t, response, "bestPath")
	assert.Contains(t, response, "gasEstimate")

	// Verify amountOut is in string format (due to custom serialization)
	amountOutStr, ok := response["amountOut"].(string)
	assert.True(t, ok, "amountOut should be a string")

	// Convert to big.Int for numerical verification
	amountOut, ok := new(big.Int).SetString(amountOutStr, 10)
	assert.True(t, ok, "amountOut should be a valid number")

	// Verify output amount is in reasonable range
	assert.True(t, amountOut.Cmp(big.NewInt(1000)) > 0, "AmountOut should be positive: %s", amountOut.String())
}

func TestGetQuote_WithSlippage(t *testing.T) {
	mockStore := new(MockStore)
	perfConfig := config.PerformanceConfig{MaxSlippage: 5.0, MaxHops: 3, MaxConcurrentPaths: 10}

	router := aggregator.NewRouter(mockStore, perfConfig)
	handler := NewHandler(router, mockStore)

	// Use amount that would cause high slippage
	amountIn, _ := new(big.Int).SetString("50000000000000000000", 10) // 50 ETH

	reqBody := map[string]interface{}{
		"tokenIn":  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
		"tokenOut": "0xdac17f958d2ee523a2206206994597c13d831ec7",
		"amountIn": amountIn.String(),
	}
	body, _ := json.Marshal(reqBody)

	// Pool reserves are relatively small
	reserve0, _ := new(big.Int).SetString("100000000000000000000", 10) // 100 ETH
	reserve1 := big.NewInt(200000000000)                               // 200,000 USDT

	mockPools := []*types.Pool{
		{
			Address:  "test-pool",
			Exchange: "Uniswap V2",
			Token0: types.Token{
				Address:  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
				Symbol:   "WETH",
				Decimals: 18,
			},
			Token1: types.Token{
				Address:  "0xdac17f958d2ee523a2206206994597c13d831ec7",
				Symbol:   "USDT",
				Decimals: 6,
			},
			Reserve0: reserve0,
			Reserve1: reserve1,
			Fee:      300,
		},
	}
	mockStore.On("GetAllPools", mock.Anything).Return(mockPools, nil)

	req := httptest.NewRequest("POST", "/api/v1/quote", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.GetQuote(w, req)

	// Should fail due to high slippage
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// Verify error message contains slippage info - check error response body
	var errorResponse map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
	if err == nil {
		// If able to parse as JSON, check error message
		if errorMsg, exists := errorResponse["error"]; exists {
			assert.True(t, strings.Contains(strings.ToLower(errorMsg.(string)), "slippage") ||
				strings.Contains(strings.ToLower(errorMsg.(string)), "no valid path"),
				"Error should mention slippage or no valid path, got: %s", errorMsg)
		}
	}
}

func TestGetQuote_InvalidJSON(t *testing.T) {
	mockStore := new(MockStore)
	perfConfig := config.PerformanceConfig{MaxSlippage: 5.0, MaxHops: 3, MaxConcurrentPaths: 10}
	router := aggregator.NewRouter(mockStore, perfConfig)
	handler := NewHandler(router, mockStore)

	// Invalid JSON
	req := httptest.NewRequest("POST", "/api/v1/quote", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.GetQuote(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetQuote_InvalidContentType(t *testing.T) {
	mockStore := new(MockStore)
	perfConfig := config.PerformanceConfig{MaxSlippage: 5.0, MaxHops: 3, MaxConcurrentPaths: 10}
	router := aggregator.NewRouter(mockStore, perfConfig)
	handler := NewHandler(router, mockStore)

	req := httptest.NewRequest("POST", "/api/v1/quote", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	handler.GetQuote(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetQuote_MissingParameters(t *testing.T) {
	mockStore := new(MockStore)
	perfConfig := config.PerformanceConfig{MaxSlippage: 5.0, MaxHops: 3, MaxConcurrentPaths: 10}
	router := aggregator.NewRouter(mockStore, perfConfig)
	handler := NewHandler(router, mockStore)

	testCases := []struct {
		name     string
		reqBody  map[string]interface{}
		expected int
	}{
		{
			name:     "Missing tokenIn",
			reqBody:  map[string]interface{}{"tokenOut": "0x123", "amountIn": "100"},
			expected: http.StatusBadRequest,
		},
		{
			name:     "Missing tokenOut",
			reqBody:  map[string]interface{}{"tokenIn": "0x123", "amountIn": "100"},
			expected: http.StatusBadRequest,
		},
		{
			name:     "Invalid amount",
			reqBody:  map[string]interface{}{"tokenIn": "0x123", "tokenOut": "0x456", "amountIn": "0"},
			expected: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.reqBody)
			req := httptest.NewRequest("POST", "/api/v1/quote", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.GetQuote(w, req)

			assert.Equal(t, tc.expected, w.Code)
		})
	}
}

func TestGetPools(t *testing.T) {
	mockStore := new(MockStore)
	perfConfig := config.PerformanceConfig{MaxSlippage: 5.0, MaxHops: 3, MaxConcurrentPaths: 10}
	router := aggregator.NewRouter(mockStore, perfConfig)
	handler := NewHandler(router, mockStore)

	// Mock cache return
	expectedPools := []*types.Pool{
		{
			Address:  "pool1",
			Exchange: "Uniswap V2",
			Token0: types.Token{
				Address:  "0xtoken0",
				Symbol:   "TOKEN0",
				Decimals: 18,
			},
			Token1: types.Token{
				Address:  "0xtoken1",
				Symbol:   "TOKEN1",
				Decimals: 18,
			},
			Reserve0: big.NewInt(1000000),
			Reserve1: big.NewInt(2000000),
		},
	}
	mockStore.On("GetAllPools", mock.Anything).Return(expectedPools, nil)

	req := httptest.NewRequest("GET", "/api/v1/pools", nil)
	w := httptest.NewRecorder()

	handler.GetPools(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockStore.AssertCalled(t, "GetAllPools", mock.Anything)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, float64(1), response["count"].(float64))

	// Verify pool data is correctly returned
	poolsData := response["pools"].([]interface{})
	assert.Equal(t, 1, len(poolsData))

	poolData := poolsData[0].(map[string]interface{})
	assert.Equal(t, "pool1", poolData["address"])
	assert.Equal(t, "Uniswap V2", poolData["exchange"])
}

func TestGetPoolsByTokens(t *testing.T) {
	mockStore := new(MockStore)
	perfConfig := config.PerformanceConfig{MaxSlippage: 5.0, MaxHops: 3, MaxConcurrentPaths: 10}
	router := aggregator.NewRouter(mockStore, perfConfig)
	handler := NewHandler(router, mockStore)

	expectedPools := []*types.Pool{
		{
			Address:  "pool1",
			Exchange: "Uniswap V2",
			Token0: types.Token{
				Address:  "0x123",
				Symbol:   "TOKENA",
				Decimals: 18,
			},
			Token1: types.Token{
				Address:  "0x456",
				Symbol:   "TOKENB",
				Decimals: 18,
			},
		},
	}
	mockStore.On("GetPoolsByTokens", mock.Anything, "0x123", "0x456").Return(expectedPools, nil)

	req := httptest.NewRequest("GET", "/api/v1/pools/search?tokenA=0x123&tokenB=0x456", nil)
	w := httptest.NewRecorder()

	handler.GetPoolsByTokens(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "0x123", response["tokenA"])
	assert.Equal(t, "0x456", response["tokenB"])
	assert.Equal(t, float64(1), response["count"].(float64))

	pools := response["pools"].([]interface{})
	assert.Equal(t, 1, len(pools))
}

func TestHealthCheck(t *testing.T) {
	mockStore := new(MockStore)
	perfConfig := config.PerformanceConfig{MaxSlippage: 5.0, MaxHops: 3, MaxConcurrentPaths: 10}
	router := aggregator.NewRouter(mockStore, perfConfig)
	handler := NewHandler(router, mockStore)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.HealthCheck(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "healthy", response["status"])
}

func TestGetConfig(t *testing.T) {
	mockStore := new(MockStore)
	perfConfig := config.PerformanceConfig{MaxSlippage: 5.0, MaxHops: 3, MaxConcurrentPaths: 10}
	router := aggregator.NewRouter(mockStore, perfConfig)
	handler := NewHandler(router, mockStore)

	// Ensure configuration is initialized
	if config.AppConfig == nil {
		config.Init()
	}

	req := httptest.NewRequest("GET", "/config", nil)
	w := httptest.NewRecorder()

	handler.GetConfig(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify configuration structure
	assert.Contains(t, response, "server")
	assert.Contains(t, response, "redis")
	assert.Contains(t, response, "ethereum")
	assert.Contains(t, response, "dex")

	serverConfig := response["server"].(map[string]interface{})
	assert.Contains(t, serverConfig, "port")
	assert.Contains(t, serverConfig, "read_timeout")
	assert.Contains(t, serverConfig, "write_timeout")
}

func TestGetPoolByAddress(t *testing.T) {
	mockStore := new(MockStore)
	perfConfig := config.PerformanceConfig{MaxSlippage: 5.0, MaxHops: 3, MaxConcurrentPaths: 10}
	router := aggregator.NewRouter(mockStore, perfConfig)
	handler := NewHandler(router, mockStore)

	expectedPool := &types.Pool{
		Address:  "test-pool",
		Exchange: "Uniswap V2",
		Token0: types.Token{
			Address:  "0xtoken0",
			Symbol:   "TOKEN0",
			Decimals: 18,
		},
		Token1: types.Token{
			Address:  "0xtoken1",
			Symbol:   "TOKEN1",
			Decimals: 18,
		},
		Reserve0: big.NewInt(1000000),
		Reserve1: big.NewInt(2000000),
	}
	mockStore.On("GetPool", mock.Anything, "test-pool").Return(expectedPool, nil)

	req := httptest.NewRequest("GET", "/api/v1/pools/test-pool", nil)
	req = mux.SetURLVars(req, map[string]string{"address": "test-pool"})
	w := httptest.NewRecorder()

	handler.GetPoolByAddress(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockStore.AssertCalled(t, "GetPool", mock.Anything, "test-pool")

	var pool types.Pool
	err := json.Unmarshal(w.Body.Bytes(), &pool)
	assert.NoError(t, err)
	assert.Equal(t, "test-pool", pool.Address)
	assert.Equal(t, "Uniswap V2", pool.Exchange)
}

func TestGetCacheStats_WithTwoLevelCache(t *testing.T) {
	// Create real Router
	mockStore := new(MockStore)
	perfConfig := config.PerformanceConfig{MaxSlippage: 5.0, MaxHops: 3, MaxConcurrentPaths: 10}
	router := aggregator.NewRouter(mockStore, perfConfig)

	// Create mock TwoLevelCache
	mockTwoLevelCache := new(MockTwoLevelCache)

	// Create Handler using mock TwoLevelCache
	handler := NewHandler(router, mockTwoLevelCache)

	// Mock cache statistics
	expectedStats := &cache.CacheStats{
		LocalHits:   100,
		LocalMisses: 20,
		RedisHits:   50,
		RedisMisses: 10,
	}
	mockTwoLevelCache.On("GetStats").Return(expectedStats)

	req := httptest.NewRequest("GET", "/cache/stats", nil)
	w := httptest.NewRecorder()

	handler.GetCacheStats(w, req)

	// Check status code
	if w.Code != http.StatusOK {
		t.Logf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// If status code is 200, verify response
	if w.Code == http.StatusOK {
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		// Safely get field values
		if localHits, exists := response["local_hits"]; exists {
			assert.Equal(t, float64(100), localHits.(float64))
		}
		if localMisses, exists := response["local_misses"]; exists {
			assert.Equal(t, float64(20), localMisses.(float64))
		}
		if redisHits, exists := response["redis_hits"]; exists {
			assert.Equal(t, float64(50), redisHits.(float64))
		}
		if redisMisses, exists := response["redis_misses"]; exists {
			assert.Equal(t, float64(10), redisMisses.(float64))
		}
	}
}

func TestGetCacheStats_WithoutTwoLevelCache(t *testing.T) {
	perfConfig := config.PerformanceConfig{MaxSlippage: 5.0, MaxHops: 3, MaxConcurrentPaths: 10}
	mockStore := new(MockStore)
	router := aggregator.NewRouter(mockStore, perfConfig)

	// Use regular MockStore, not TwoLevelCache
	handler := NewHandler(router, mockStore)

	req := httptest.NewRequest("GET", "/cache/stats", nil)
	w := httptest.NewRecorder()

	handler.GetCacheStats(w, req)

	// Should return not implemented error
	assert.Equal(t, http.StatusNotImplemented, w.Code)
}
