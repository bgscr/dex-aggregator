package aggregator

import (
	"fmt"
	"math/big"

	"dex-aggregator/internal/types"
)

type PriceCalculator struct{}

func NewPriceCalculator() *PriceCalculator {
	return &PriceCalculator{}
}

// Calculate single pool output amount
func (pc *PriceCalculator) CalculateOutput(pool *types.Pool, amountIn *big.Int, tokenIn string) (*big.Int, error) {
	var reserveIn, reserveOut *big.Int

	// Determine input and output reserves
	if pool.Token0.Address == tokenIn {
		reserveIn = pool.Reserve0
		reserveOut = pool.Reserve1
	} else if pool.Token1.Address == tokenIn {
		reserveIn = pool.Reserve1
		reserveOut = pool.Reserve0
	} else {
		return big.NewInt(0), fmt.Errorf("token %s not found in pool", tokenIn)
	}

	// Check reserves
	if reserveIn.Cmp(big.NewInt(0)) == 0 || reserveOut.Cmp(big.NewInt(0)) == 0 {
		return big.NewInt(0), nil
	}

	// Uniswap V2 constant product formula
	// amountOut = (reserveOut * amountIn * 997) / (reserveIn * 1000 + amountIn * 997)

	fee := big.NewInt(997) // 0.3% fee
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

// Calculate path output amount
func (pc *PriceCalculator) CalculatePathOutput(pools []*types.Pool, amountIn *big.Int, tokenIn, tokenOut string) (*big.Int, error) {
	if len(pools) == 0 {
		return big.NewInt(0), nil
	}

	currentAmount := new(big.Int).Set(amountIn)
	currentToken := tokenIn

	for i, pool := range pools {
		// For each pool, determine the correct input token
		var inputToken string

		if i == 0 {
			// First pool: use the requested tokenIn
			inputToken = tokenIn
		} else {
			// Subsequent pools: use the output token from previous pool
			inputToken = currentToken
		}

		// Calculate output for this pool
		amountOut, err := pc.CalculateOutput(pool, currentAmount, inputToken)
		if err != nil {
			return big.NewInt(0), fmt.Errorf("pool %d calculation failed: %v", i, err)
		}

		// Determine output token for this pool
		if pool.Token0.Address == inputToken {
			currentToken = pool.Token1.Address
		} else {
			currentToken = pool.Token0.Address
		}

		currentAmount = amountOut

		// If this is the last pool, verify output token matches requested tokenOut
		if i == len(pools)-1 {
			if currentToken != tokenOut {
				return big.NewInt(0), fmt.Errorf("final output token %s does not match requested tokenOut %s", currentToken, tokenOut)
			}
		}
	}

	return currentAmount, nil
}
