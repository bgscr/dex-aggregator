package aggregator

import (
	"fmt"
	"log"
	"math/big"
	"strings"

	"dex-aggregator/internal/types"
)

type PriceCalculator struct {
	maxSlippage float64 // Maximum allowed slippage percentage
}

func NewPriceCalculator() *PriceCalculator {
	return &PriceCalculator{
		maxSlippage: 5.0,
	}
}

// CalculateOutput calculates output amount for a single pool with slippage check
func (pc *PriceCalculator) CalculateOutput(pool *types.Pool, amountIn *big.Int, tokenIn string) (*big.Int, error) {
	var reserveIn, reserveOut *big.Int

	poolToken0 := strings.ToLower(pool.Token0.Address)
	poolToken1 := strings.ToLower(pool.Token1.Address)
	tokenInLower := strings.ToLower(tokenIn)

	log.Printf("CalculateOutput: pool %s, tokens: %s/%s, input token: %s",
		pool.Address, poolToken0, poolToken1, tokenInLower)

	if poolToken0 == tokenInLower {
		reserveIn = pool.Reserve0
		reserveOut = pool.Reserve1
		log.Printf("Token0 match, reserves: in=%s, out=%s", reserveIn.String(), reserveOut.String())
	} else if poolToken1 == tokenInLower {
		reserveIn = pool.Reserve1
		reserveOut = pool.Reserve0
		log.Printf("Token1 match, reserves: in=%s, out=%s", reserveIn.String(), reserveOut.String())
	} else {
		log.Printf("Token %s not found in pool", tokenIn)
		return big.NewInt(0), fmt.Errorf("token %s not found in pool", tokenIn)
	}

	if reserveIn.Cmp(big.NewInt(0)) == 0 || reserveOut.Cmp(big.NewInt(0)) == 0 {
		log.Printf("Zero reserves: in=%s, out=%s", reserveIn.String(), reserveOut.String())
		return big.NewInt(0), nil
	}

	if err := pc.checkSlippage(reserveIn, reserveOut, amountIn); err != nil {
		log.Printf("Slippage check failed: %v", err)
		return big.NewInt(0), err
	}

	fee := big.NewInt(997)
	thousand := big.NewInt(1000)

	amountInWithFee := new(big.Int).Mul(amountIn, fee)
	numerator := new(big.Int).Mul(reserveOut, amountInWithFee)

	denominator := new(big.Int).Mul(reserveIn, thousand)
	denominator.Add(denominator, amountInWithFee)

	if denominator.Cmp(big.NewInt(0)) == 0 {
		log.Printf("Zero denominator")
		return big.NewInt(0), nil
	}

	amountOut := new(big.Int).Div(numerator, denominator)

	log.Printf("Calculation: amountIn=%s, amountOut=%s", amountIn.String(), amountOut.String())

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
	currentToken := strings.ToLower(tokenIn)
	tokenOutLower := strings.ToLower(tokenOut)

	for i, pool := range pools {
		inputToken := currentToken

		amountOut, err := pc.CalculateOutput(pool, currentAmount, inputToken)
		if err != nil {
			return big.NewInt(0), fmt.Errorf("pool %d calculation failed: %v", i, err)
		}

		poolToken0Lower := strings.ToLower(pool.Token0.Address)
		poolToken1Lower := strings.ToLower(pool.Token1.Address)
		inputTokenLower := strings.ToLower(inputToken)

		if poolToken0Lower == inputTokenLower {
			currentToken = poolToken1Lower
		} else if poolToken1Lower == inputTokenLower {
			currentToken = poolToken0Lower
		} else {
			return big.NewInt(0), fmt.Errorf("token %s not found in pool %s", inputToken, pool.Address)
		}

		currentAmount = amountOut

		if i == len(pools)-1 {
			if currentToken != tokenOutLower {
				return big.NewInt(0), fmt.Errorf("final output token %s does not match requested tokenOut %s", currentToken, tokenOutLower)
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

	// 1. Convert to float for high precision calculation
	fReserveIn := new(big.Float).SetInt(reserveIn)
	fReserveOut := new(big.Float).SetInt(reserveOut)
	fAmountIn := new(big.Float).SetInt(amountIn)

	// 2. Check division by zero
	if fReserveIn.Cmp(big.NewFloat(0)) == 0 {
		log.Printf("Slippage check: zero reserveIn")
		return fmt.Errorf("zero reserveIn")
	}

	// 3. Calculate spot price (before trade)
	// spotPrice = reserveOut / reserveIn
	spotPrice := new(big.Float).Quo(fReserveOut, fReserveIn)
	if spotPrice.Cmp(big.NewFloat(0)) == 0 {
		return fmt.Errorf("zero spot price")
	}

	// 4. Calculate actual received amountOut (including fee)
	amountOut := calculateOutputWithFee(reserveIn, reserveOut, amountIn)
	fAmountOut := new(big.Float).SetInt(amountOut)

	// 5. Calculate effective price
	// effectivePrice = amountOut / amountIn
	if fAmountIn.Cmp(big.NewFloat(0)) == 0 {
		return fmt.Errorf("zero amountIn")
	}
	effectivePrice := new(big.Float).Quo(fAmountOut, fAmountIn)

	// 6. Calculate price impact
	// impact = (spotPrice - effectivePrice) / spotPrice
	priceImpact := new(big.Float).Sub(spotPrice, effectivePrice)
	priceImpactRatio := new(big.Float).Quo(priceImpact, spotPrice)

	// 7. Convert to percentage
	slippagePercent, _ := priceImpactRatio.Float64()
	slippagePercent = slippagePercent * 100

	log.Printf("Slippage check: Spot=%.6f, Eff=%.6f, Impact=%.2f%% (Max: %.2f%%)",
		spotPrice, effectivePrice, slippagePercent, maxSlippage)

	// 8. Check if exceeds maximum allowed slippage
	if slippagePercent > maxSlippage {
		return fmt.Errorf("slippage too high: %.2f%% (max: %.2f%%)", slippagePercent, maxSlippage)
	}

	log.Printf("Slippage check passed: %.2f%%", slippagePercent)
	return nil
}

func calculateOutputWithoutFee(reserveIn, reserveOut, amountIn *big.Int) *big.Int {
	numerator := new(big.Int).Mul(reserveOut, amountIn)
	denominator := new(big.Int).Add(reserveIn, amountIn)

	if denominator.Cmp(big.NewInt(0)) == 0 {
		return big.NewInt(0)
	}

	return new(big.Int).Div(numerator, denominator)
}

func calculateOutputWithFee(reserveIn, reserveOut, amountIn *big.Int) *big.Int {
	fee := big.NewInt(997)
	thousand := big.NewInt(1000)

	amountInWithFee := new(big.Int).Mul(amountIn, fee)
	numerator := new(big.Int).Mul(reserveOut, amountInWithFee)

	denominator := new(big.Int).Mul(reserveIn, thousand)
	denominator.Add(denominator, amountInWithFee)

	if denominator.Cmp(big.NewInt(0)) == 0 {
		return big.NewInt(0)
	}

	return new(big.Int).Div(numerator, denominator)
}

// SetMaxSlippage updates the maximum allowed slippage
func (pc *PriceCalculator) SetMaxSlippage(slippage float64) {
	pc.maxSlippage = slippage
}
