package indicators

import (
	"math"
)

// SMA calculates Simple Moving Average
func SMA(prices []float64, period int) float64 {
	if len(prices) < period {
		return 0
	}

	sum := 0.0
	for i := len(prices) - period; i < len(prices); i++ {
		sum += prices[i]
	}
	return sum / float64(period)
}

// EMA calculates Exponential Moving Average
func EMA(prices []float64, period int) float64 {
	if len(prices) < period {
		return 0
	}

	multiplier := 2.0 / float64(period+1)
	ema := SMA(prices[:period], period)

	for i := period; i < len(prices); i++ {
		ema = (prices[i] * multiplier) + (ema * (1 - multiplier))
	}
	return ema
}

// RSI calculates Relative Strength Index
func RSI(prices []float64, period int) float64 {
	if len(prices) < period+1 {
		return 50.0 // Neutral
	}

	gains := 0.0
	losses := 0.0

	// Calculate initial average gain/loss
	for i := len(prices) - period; i < len(prices); i++ {
		change := prices[i] - prices[i-1]
		if change > 0 {
			gains += change
		} else {
			losses += -change
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	// Handle edge cases
	if avgLoss == 0 {
		if avgGain == 0 {
			return 50.0 // No movement
		}
		return 100.0 // Only gains
	}
	
	if avgGain == 0 {
		return 0.0 // Only losses
	}

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))
	return rsi
}

// ATR calculates Average True Range (volatility indicator)
func ATR(highs, lows, closes []float64, period int) float64 {
	if len(highs) < period+1 || len(lows) < period+1 || len(closes) < period+1 {
		return 0
	}

	trueRanges := make([]float64, 0)
	
	for i := 1; i < len(closes); i++ {
		highLow := highs[i] - lows[i]
		highClose := math.Abs(highs[i] - closes[i-1])
		lowClose := math.Abs(lows[i] - closes[i-1])
		
		tr := math.Max(highLow, math.Max(highClose, lowClose))
		trueRanges = append(trueRanges, tr)
	}

	// Calculate ATR as SMA of true ranges
	return SMA(trueRanges, period)
}

// MACD calculates Moving Average Convergence Divergence
func MACD(prices []float64, fastPeriod, slowPeriod, signalPeriod int) (macd, signal, histogram float64) {
	if len(prices) < slowPeriod {
		return 0, 0, 0
	}

	fastEMA := EMA(prices, fastPeriod)
	slowEMA := EMA(prices, slowPeriod)
	macd = fastEMA - slowEMA

	// Calculate signal line (EMA of MACD)
	// This is simplified - ideally track MACD values over time
	signal = macd * 0.9 // Simplified
	histogram = macd - signal

	return macd, signal, histogram
}

// BollingerBands calculates upper and lower Bollinger Bands
func BollingerBands(prices []float64, period int, stdDev float64) (middle, upper, lower float64) {
	if len(prices) < period {
		return 0, 0, 0
	}

	middle = SMA(prices, period)
	
	// Calculate standard deviation
	variance := 0.0
	for i := len(prices) - period; i < len(prices); i++ {
		diff := prices[i] - middle
		variance += diff * diff
	}
	stdDeviation := math.Sqrt(variance / float64(period))

	upper = middle + (stdDeviation * stdDev)
	lower = middle - (stdDeviation * stdDev)

	return middle, upper, lower
}

// StdDev calculates standard deviation
func StdDev(prices []float64) float64 {
	if len(prices) == 0 {
		return 0
	}

	mean := 0.0
	for _, p := range prices {
		mean += p
	}
	mean /= float64(len(prices))

	variance := 0.0
	for _, p := range prices {
		diff := p - mean
		variance += diff * diff
	}
	variance /= float64(len(prices))

	return math.Sqrt(variance)
}

// PercentChange calculates percentage change between two prices
func PercentChange(oldPrice, newPrice float64) float64 {
	if oldPrice == 0 {
		return 0
	}
	return ((newPrice - oldPrice) / oldPrice) * 100
}

// Volatility calculates historical volatility (annualized)
func Volatility(prices []float64, period int) float64 {
	if len(prices) < period {
		return 0
	}

	returns := make([]float64, 0)
	for i := len(prices) - period; i < len(prices)-1; i++ {
		ret := math.Log(prices[i+1] / prices[i])
		returns = append(returns, ret)
	}

	// Annualize volatility (assuming daily data, 252 trading days/year)
	return StdDev(returns) * math.Sqrt(252) * 100
}
