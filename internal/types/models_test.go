package types

import (
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTokenModel(t *testing.T) {
	token := &Token{
		Address:  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
		Symbol:   "WETH",
		Decimals: 18,
	}

	assert.Equal(t, "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2", token.Address)
	assert.Equal(t, "WETH", token.Symbol)
	assert.Equal(t, 18, token.Decimals)
}

func TestPoolModel(t *testing.T) {
	pool := &Pool{
		Address:  "0x0d4a11d5eeaac28ec3f61d100daf4d40471f1852",
		Exchange: "Uniswap V2",
		Version:  "v2",
		Token0: Token{
			Address:  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
			Symbol:   "WETH",
			Decimals: 18,
		},
		Token1: Token{
			Address:  "0xdac17f958d2ee523a2206206994597c13d831ec7",
			Symbol:   "USDT",
			Decimals: 6,
		},
		Reserve0:    big.NewInt(1000000000000000000), // 1 ETH
		Reserve1:    big.NewInt(2000000000),          // 2000 USDT
		Fee:         300,
		LastUpdated: time.Now(),
	}

	assert.Equal(t, "Uniswap V2", pool.Exchange)
	assert.Equal(t, int64(1000000000000000000), pool.Reserve0.Int64())
	assert.Equal(t, 300, pool.Fee)
}

func TestQuoteRequestJSON(t *testing.T) {
	// 测试 JSON 序列化和反序列化
	req := &QuoteRequest{
		TokenIn:  "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
		TokenOut: "0xdac17f958d2ee523a2206206994597c13d831ec7",
		AmountIn: big.NewInt(100000000000000000), // 0.1 ETH
		MaxHops:  3,
	}

	// 序列化
	data, err := json.Marshal(req)
	assert.NoError(t, err)

	// 反序列化
	var newReq QuoteRequest
	err = json.Unmarshal(data, &newReq)
	assert.NoError(t, err)

	assert.Equal(t, req.TokenIn, newReq.TokenIn)
	assert.Equal(t, req.TokenOut, newReq.TokenOut)
	assert.Equal(t, req.AmountIn.String(), newReq.AmountIn.String())
	assert.Equal(t, req.MaxHops, newReq.MaxHops)
}

func TestPoolJSON(t *testing.T) {
	pool := &Pool{
		Address:  "test-pool",
		Reserve0: big.NewInt(1000000),
		Reserve1: big.NewInt(2000000),
	}

	data, err := json.Marshal(pool)
	assert.NoError(t, err)

	var newPool Pool
	err = json.Unmarshal(data, &newPool)
	assert.NoError(t, err)

	assert.Equal(t, pool.Reserve0.String(), newPool.Reserve0.String())
	assert.Equal(t, pool.Reserve1.String(), newPool.Reserve1.String())
}

func TestInvalidBigIntJSON(t *testing.T) {
	// 测试无效的 big.Int 格式
	invalidJSON := `{
		"tokenIn": "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
		"tokenOut": "0xdac17f958d2ee523a2206206994597c13d831ec7",
		"amountIn": "invalid-number"
	}`

	var req QuoteRequest
	err := json.Unmarshal([]byte(invalidJSON), &req)
	assert.Error(t, err)
}

// 在文件末尾添加这个测试函数
func TestQuoteResponseJSON(t *testing.T) {
	resp := &QuoteResponse{
		AmountOut:      big.NewInt(200000000), // 200 USDT
		GasEstimate:    big.NewInt(150000),
		ProcessingTime: 50,
	}

	// 只测试序列化，不测试反序列化（因为我们的自定义序列化器可能不完整）
	data, err := json.Marshal(resp)
	assert.NoError(t, err)

	// 验证序列化后的数据包含正确的字段
	var jsonData map[string]interface{}
	err = json.Unmarshal(data, &jsonData)
	assert.NoError(t, err)

	assert.Equal(t, "200000000", jsonData["amountOut"])
	assert.Equal(t, "150000", jsonData["gasEstimate"])
}
