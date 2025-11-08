package types

import (
	"encoding/json"
	"fmt"
	"math/big"
	"time"
)

// Token information
type Token struct {
	Address  string `json:"address" bson:"address"`
	Symbol   string `json:"symbol" bson:"symbol"`
	Decimals int    `json:"decimals" bson:"decimals"`
}

// Liquidity pool
type Pool struct {
	Address     string    `json:"address" bson:"address"`
	Exchange    string    `json:"exchange" bson:"exchange"`
	Version     string    `json:"version" bson:"version"`
	Token0      Token     `json:"token0" bson:"token0"`
	Token1      Token     `json:"token1" bson:"token1"`
	Reserve0    *big.Int  `json:"reserve0" bson:"reserve0"`
	Reserve1    *big.Int  `json:"reserve1" bson:"reserve1"`
	Fee         int       `json:"fee" bson:"fee"`
	LastUpdated time.Time `json:"last_updated" bson:"last_updated"`
}

// DEX exchange configuration
type Exchange struct {
	Name    string `json:"name" bson:"name"`
	Factory string `json:"factory" bson:"factory"`
	Router  string `json:"router" bson:"router"`
	Version string `json:"version" bson:"version"`
}

// QuoteRequest request for price quote
type QuoteRequest struct {
	TokenIn  string   `json:"tokenIn"`
	TokenOut string   `json:"tokenOut"`
	AmountIn *big.Int `json:"amountIn"`
	MaxHops  int      `json:"maxHops,omitempty"`
}

// UnmarshalJSON custom unmarshaler for QuoteRequest to handle big.Int
func (q *QuoteRequest) UnmarshalJSON(data []byte) error {
	type Alias QuoteRequest
	aux := &struct {
		AmountIn string `json:"amountIn"`
		*Alias
	}{
		Alias: (*Alias)(q),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Convert string to big.Int
	if aux.AmountIn != "" {
		amount, ok := new(big.Int).SetString(aux.AmountIn, 10)
		if !ok {
			return fmt.Errorf("invalid amountIn format: %s", aux.AmountIn)
		}
		q.AmountIn = amount
	}

	return nil
}

// MarshalJSON custom marshaler for QuoteRequest to handle big.Int
func (q *QuoteRequest) MarshalJSON() ([]byte, error) {
	type Alias QuoteRequest
	return json.Marshal(&struct {
		AmountIn string `json:"amountIn"`
		*Alias
	}{
		AmountIn: q.AmountIn.String(),
		Alias:    (*Alias)(q),
	})
}

// QuoteResponse response for price quote
type QuoteResponse struct {
	AmountOut      *big.Int     `json:"amountOut"`
	Paths          []*TradePath `json:"paths"`
	BestPath       *TradePath   `json:"bestPath"`
	GasEstimate    *big.Int     `json:"gasEstimate"`
	ProcessingTime int64        `json:"processingTime,omitempty"` // Processing time in milliseconds
}

// MarshalJSON custom marshaler for QuoteResponse to handle big.Int
func (q *QuoteResponse) MarshalJSON() ([]byte, error) {
	type Alias QuoteResponse
	return json.Marshal(&struct {
		AmountOut   string `json:"amountOut"`
		GasEstimate string `json:"gasEstimate"`
		*Alias
	}{
		AmountOut:   q.AmountOut.String(),
		GasEstimate: q.GasEstimate.String(),
		Alias:       (*Alias)(q),
	})
}

// UnmarshalJSON custom unmarshaler for QuoteResponse to handle big.Int
func (q *QuoteResponse) UnmarshalJSON(data []byte) error {
	type Alias QuoteResponse
	aux := &struct {
		AmountOut   string `json:"amountOut"`
		GasEstimate string `json:"gasEstimate"`
		*Alias
	}{
		Alias: (*Alias)(q),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Convert strings to big.Int
	if aux.AmountOut != "" {
		amountOut, ok := new(big.Int).SetString(aux.AmountOut, 10)
		if !ok {
			return fmt.Errorf("invalid amountOut format: %s", aux.AmountOut)
		}
		q.AmountOut = amountOut
	}

	if aux.GasEstimate != "" {
		gasEstimate, ok := new(big.Int).SetString(aux.GasEstimate, 10)
		if !ok {
			return fmt.Errorf("invalid gasEstimate format: %s", aux.GasEstimate)
		}
		q.GasEstimate = gasEstimate
	}

	return nil
}

// TradePath trading path
type TradePath struct {
	Pools     []*Pool  `json:"pools"`
	AmountOut *big.Int `json:"amountOut"`
	Dexes     []string `json:"dexes"`
	GasCost   *big.Int `json:"gasCost"`
}

// MarshalJSON custom marshaler for TradePath to handle big.Int
func (t *TradePath) MarshalJSON() ([]byte, error) {
	type Alias TradePath
	return json.Marshal(&struct {
		AmountOut string `json:"amountOut"`
		GasCost   string `json:"gasCost"`
		*Alias
	}{
		AmountOut: t.AmountOut.String(),
		GasCost:   t.GasCost.String(),
		Alias:     (*Alias)(t),
	})
}

// MarshalJSON custom marshaler for Pool to handle big.Int
func (p *Pool) MarshalJSON() ([]byte, error) {
	type Alias Pool
	return json.Marshal(&struct {
		Reserve0 string `json:"reserve0"`
		Reserve1 string `json:"reserve1"`
		*Alias
	}{
		Reserve0: p.Reserve0.String(),
		Reserve1: p.Reserve1.String(),
		Alias:    (*Alias)(p),
	})
}

// UnmarshalJSON custom unmarshaler for Pool to handle big.Int
func (p *Pool) UnmarshalJSON(data []byte) error {
	type Alias Pool
	aux := &struct {
		Reserve0 string `json:"reserve0"`
		Reserve1 string `json:"reserve1"`
		*Alias
	}{
		Alias: (*Alias)(p),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Convert strings to big.Int
	if aux.Reserve0 != "" {
		reserve0, ok := new(big.Int).SetString(aux.Reserve0, 10)
		if !ok {
			return fmt.Errorf("invalid reserve0 format: %s", aux.Reserve0)
		}
		p.Reserve0 = reserve0
	}

	if aux.Reserve1 != "" {
		reserve1, ok := new(big.Int).SetString(aux.Reserve1, 10)
		if !ok {
			return fmt.Errorf("invalid reserve1 format: %s", aux.Reserve1)
		}
		p.Reserve1 = reserve1
	}

	return nil
}
