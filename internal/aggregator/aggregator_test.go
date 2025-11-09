package aggregator

import (
	"context"
	"dex-aggregator/config"
	"dex-aggregator/internal/types"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockStore for testing
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
	// FIX: Changed (*types.Pool) to ([]*types.Pool) to match interface
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

func TestPriceCalculator_CalculateOutput(t *testing.T) {
	calculator := NewPriceCalculator()
	calculator.SetMaxSlippage(5.0) // Temporarily increase slippage limit for testing

	pool := &types.Pool{
		Token0:   types.Token{Address: "0xTokenA"},
		Token1:   types.Token{Address: "0xTokenB"},
		Reserve0: big.NewInt(1000000000), // Larger reserves to reduce slippage
		Reserve1: big.NewInt(2000000000),
	}

	// Use smaller input amount to avoid slippage errors
	amountOut, err := calculator.CalculateOutput(pool, big.NewInt(1000), "0xTokenA")
	assert.NoError(t, err)
	assert.True(t, amountOut.Cmp(big.NewInt(0)) > 0)

	// Test non-existent token
	_, err = calculator.CalculateOutput(pool, big.NewInt(1000), "0xInvalidToken")
	assert.Error(t, err)

	// Test zero reserves
	zeroPool := &types.Pool{
		Token0:   types.Token{Address: "0xTokenA"},
		Token1:   types.Token{Address: "0xTokenB"},
		Reserve0: big.NewInt(0),
		Reserve1: big.NewInt(0),
	}
	amountOut, err = calculator.CalculateOutput(zeroPool, big.NewInt(1000), "0xTokenA")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), amountOut.Int64())
}

func TestPriceCalculator_CalculatePathOutput(t *testing.T) {
	calculator := NewPriceCalculator()
	calculator.SetMaxSlippage(5.0) // Temporarily increase slippage limit

	pools := []*types.Pool{
		{
			Token0:   types.Token{Address: "0xTokenA"},
			Token1:   types.Token{Address: "0xTokenB"},
			Reserve0: big.NewInt(1000000000),
			Reserve1: big.NewInt(2000000000),
		},
		{
			Token0:   types.Token{Address: "0xTokenB"},
			Token1:   types.Token{Address: "0xTokenC"},
			Reserve0: big.NewInt(1500000000),
			Reserve1: big.NewInt(3000000000),
		},
	}

	amountOut, err := calculator.CalculatePathOutput(pools, big.NewInt(1000), "0xTokenA", "0xTokenC")
	assert.NoError(t, err)
	assert.True(t, amountOut.Cmp(big.NewInt(0)) > 0)

	// Test empty path
	amountOut, err = calculator.CalculatePathOutput([]*types.Pool{}, big.NewInt(1000), "0xTokenA", "0xTokenB")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), amountOut.Int64())
}

func TestRouter_GetBestQuote(t *testing.T) {
	perfConfig := config.PerformanceConfig{
		MaxSlippage:        5.0,
		MaxHops:            3,
		MaxConcurrentPaths: 10,
	}
	mockStore := new(MockStore)

	// Mock pools returned by cache
	mockPools := []*types.Pool{
		{
			Address:  "pool1",
			Exchange: "Uniswap V2",
			Token0:   types.Token{Address: "0xweth"},
			Token1:   types.Token{Address: "0xusdt"},
			Reserve0: big.NewInt(1000000000000000000), // 1 ETH
			Reserve1: big.NewInt(2000000000000),       // Larger USDT reserves
		},
	}

	// Set expectation *BEFORE* NewRouter is called.
	// It's called twice: 1. By NewPathFinder (initial load), 2. By GetBestQuote (logging).
	mockStore.On("GetAllPools", mock.Anything).Return(mockPools, nil).Twice()

	router := NewRouter(mockStore, perfConfig)

	req := &types.QuoteRequest{
		TokenIn:  "0xweth",
		TokenOut: "0xusdt",
		AmountIn: big.NewInt(1000000000000000), // Smaller input amount 0.001 ETH
		MaxHops:  3,
	}

	response, err := router.GetBestQuote(context.Background(), req)

	// We now expect a valid path
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.AmountOut.Cmp(big.NewInt(0)) > 0)

	mockStore.AssertExpectations(t)
}

func TestPathFinder_FindDirectPaths(t *testing.T) {
	mockStore := new(MockStore)

	// Build test data - ensure sufficient liquidity
	mockPools := []*types.Pool{
		{
			Address:  "pool1",
			Token0:   types.Token{Address: "0xtokena"},
			Token1:   types.Token{Address: "0xtokenb"},
			Reserve0: big.NewInt(1000000000),
			Reserve1: big.NewInt(2000000000),
		},
		{
			Address:  "pool2",
			Token0:   types.Token{Address: "0xtokena"},
			Token1:   types.Token{Address: "0xtokenb"},
			Reserve0: big.NewInt(1000000000),
			Reserve1: big.NewInt(2000000000),
		},
	}

	// This mock is for the *initial load* in NewPathFinder
	// It must be set *BEFORE* NewPathFinder is called.
	mockStore.On("GetAllPools", mock.Anything).Return(mockPools, nil).Once()

	// Now, create the PathFinder
	pathFinder := NewPathFinder(mockStore, NewPriceCalculator())

	// We can optionally test the RefreshGraph function again explicitly
	// Add a second expectation for this explicit call.
	mockStore.On("GetAllPools", mock.Anything).Return(mockPools, nil).Once()
	err := pathFinder.RefreshGraph(context.Background())
	assert.NoError(t, err)

	paths, err := pathFinder.FindBestPaths(context.Background(), "0xtokena", "0xtokenb", big.NewInt(1000), 3, 10)
	assert.NoError(t, err)
	assert.Greater(t, len(paths), 0)

	mockStore.AssertExpectations(t)
}

func TestRouter_FindOptimalPath(t *testing.T) {
	perfConfig := config.PerformanceConfig{
		MaxSlippage:        5.0,
		MaxHops:            3,
		MaxConcurrentPaths: 10,
	}
	mockStore := new(MockStore)

	// Add expectation for the initial load in NewRouter
	mockStore.On("GetAllPools", mock.Anything).Return([]*types.Pool{}, nil).Once()

	router := NewRouter(mockStore, perfConfig)

	tradePaths := []*types.TradePath{
		{
			AmountOut: big.NewInt(1000),
			GasCost:   big.NewInt(100),
		},
		{
			AmountOut: big.NewInt(1200),
			GasCost:   big.NewInt(150),
		},
		{
			AmountOut: big.NewInt(900),
			GasCost:   big.NewInt(50),
		},
	}

	bestPath := router.findOptimalPath(tradePaths)
	assert.NotNil(t, bestPath)
	assert.Equal(t, int64(1200), bestPath.AmountOut.Int64())

	mockStore.AssertExpectations(t)
}
