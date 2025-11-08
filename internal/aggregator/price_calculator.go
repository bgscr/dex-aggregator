package aggregator

import (
	"fmt"
	"math/big"

	"dex-aggregator/internal/types"
)

type PriceCalculator struct {
	maxSlippage float64 // Maximum allowed slippage percentage
}

func NewPriceCalculator() *PriceCalculator {
	return &PriceCalculator{
		maxSlippage: 5.0, // 5% maximum slippage
	}
}

// CalculateOutput calculates output amount for a single pool with slippage check
func (pc *PriceCalculator) CalculateOutput(pool *types.Pool, amountIn *big.Int, tokenIn string) (*big.Int, error) {
	var reserveIn, reserveOut *big.Int

	if pool.Token0.Address == tokenIn {
		reserveIn = pool.Reserve0
		reserveOut = pool.Reserve1
	} else if pool.Token1.Address == tokenIn {
		reserveIn = pool.Reserve1
		reserveOut = pool.Reserve0
	} else {
		return big.NewInt(0), fmt.Errorf("token %s not found in pool", tokenIn)
	}

	if reserveIn.Cmp(big.NewInt(0)) == 0 || reserveOut.Cmp(big.NewInt(0)) == 0 {
		return big.NewInt(0), nil
	}

	// Check if the trade would cause too much slippage
	if err := pc.checkSlippage(reserveIn, reserveOut, amountIn); err != nil {
		return big.NewInt(0), err
	}

	fee := big.NewInt(997)
	thousand := big.NewInt(1000)

	amountInWithFee := new(big.Int).Mul(amountIn, fee)
	numerator := new(big.Int).Mul(reserveOut, amountInWithFee)

	denominator := new(big.Int).Mul(reserveIn, thousand)
	denominator.Add(denominator, amountInWithFee)

	if denominator.Cmp(big.NewInt(0)) == 0 {
		return big.NewInt(0), nil
	}

	amountOut := new(big.Int).Div(numerator, denominator)

	return amountOut, nil
}

// CalculateOutputWithSlippageCheck calculates output with custom slippage limit
func (pc *PriceCalculator) CalculateOutputWithSlippageCheck(pool *types.Pool, amountIn *big.Int, tokenIn string, maxSlippage float64) (*big.Int, error) {
	var reserveIn, reserveOut *big.Int

	if pool.Token0.Address == tokenIn {
		reserveIn = pool.Reserve0
		reserveOut = pool.Reserve1
	} else if pool.Token1.Address == tokenIn {
		reserveIn = pool.Reserve1
		reserveOut = pool.Reserve0
	} else {
		return big.NewInt(0), fmt.Errorf("token %s not found in pool", tokenIn)
	}

	if reserveIn.Cmp(big.NewInt(0)) == 0 || reserveOut.Cmp(big.NewInt(0)) == 0 {
		return big.NewInt(0), nil
	}

	// Check slippage with custom limit
	if err := pc.checkSlippageWithLimit(reserveIn, reserveOut, amountIn, maxSlippage); err != nil {
		return big.NewInt(0), err
	}

	fee := big.NewInt(997)
	thousand := big.NewInt(1000)

	amountInWithFee := new(big.Int).Mul(amountIn, fee)
	numerator := new(big.Int).Mul(reserveOut, amountInWithFee)

	denominator := new(big.Int).Mul(reserveIn, thousand)
	denominator.Add(denominator, amountInWithFee)

	if denominator.Cmp(big.NewInt(0)) == 0 {
		return big.NewInt(0), nil
	}

	amountOut := new(big.Int).Div(numerator, denominator)

	return amountOut, nil
}

// CalculatePathOutput calculates output for a multi-hop path
func (pc *PriceCalculator) CalculatePathOutput(pools []*types.Pool, amountIn *big.Int, tokenIn, tokenOut string) (*big.Int, error) {
	if len(pools) == 0 {
		return big.NewInt(0), nil
	}

	currentAmount := new(big.Int).Set(amountIn)
	currentToken := tokenIn

	for i, pool := range pools {
		var inputToken string

		if i == 0 {
			inputToken = tokenIn
		} else {
			inputToken = currentToken
		}

		amountOut, err := pc.CalculateOutput(pool, currentAmount, inputToken)
		if err != nil {
			return big.NewInt(0), fmt.Errorf("pool %d calculation failed: %v", i, err)
		}

		if pool.Token0.Address == inputToken {
			currentToken = pool.Token1.Address
		} else {
			currentToken = pool.Token0.Address
		}

		currentAmount = amountOut

		if i == len(pools)-1 {
			if currentToken != tokenOut {
				return big.NewInt(0), fmt.Errorf("final output token %s does not match requested tokenOut %s", currentToken, tokenOut)
			}
		}
	}

	return currentAmount, nil
}

// checkSlippage verifies that the trade doesn't exceed maximum slippage
func (pc *PriceCalculator) checkSlippage(reserveIn, reserveOut, amountIn *big.Int) error {
	return pc.checkSlippageWithLimit(reserveIn, reserveOut, amountIn, pc.maxSlippage)
}

// checkSlippageWithLimit verifies slippage with custom limit
func (pc *PriceCalculator) checkSlippageWithLimit(reserveIn, reserveOut, amountIn *big.Int, maxSlippage float64) error {
	if amountIn.Cmp(big.NewInt(0)) == 0 {
		return nil
	}

	// Calculate price impact
	reserveInFloat := new(big.Float).SetInt(reserveIn)
	reserveOutFloat := new(big.Float).SetInt(reserveOut)
	amountInFloat := new(big.Float).SetInt(amountIn)

	// Original price
	originalPrice := new(big.Float).Quo(reserveOutFloat, reserveInFloat)

	// New reserves after trade
	newReserveIn := new(big.Float).Add(reserveInFloat, amountInFloat)
	feeAdjustedAmount := new(big.Float).Mul(amountInFloat, big.NewFloat(0.997)) // 0.3% fee
	newReserveOut := new(big.Float).Sub(reserveOutFloat, new(big.Float).Mul(originalPrice, feeAdjustedAmount))

	if newReserveOut.Sign() <= 0 {
		return fmt.Errorf("insufficient liquidity after trade")
	}

	// New price
	newPrice := new(big.Float).Quo(newReserveOut, newReserveIn)

	// Calculate slippage
	priceRatio := new(big.Float).Quo(newPrice, originalPrice)
	slippage := new(big.Float).Sub(big.NewFloat(1.0), priceRatio)
	slippagePercent, _ := slippage.Float64()
	slippagePercent = slippagePercent * 100 // Convert to percentage

	if slippagePercent > maxSlippage {
		return fmt.Errorf("slippage too high: %.2f%% (max: %.2f%%)", slippagePercent, maxSlippage)
	}

	return nil
}

// SetMaxSlippage updates the maximum allowed slippage
func (pc *PriceCalculator) SetMaxSlippage(slippage float64) {
	pc.maxSlippage = slippage
}
