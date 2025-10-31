package indicators

import (
	"math"

)

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

func RSI(prices []float64, period int) float64 {
	if len(prices) < period+1{
		return 50.0
	}

	gains := 0.0
	losses := 0.0

	for i := len(prices) - period; i < len(prices); i++ {
		change := prices[i] - prices[i-1]
		if change > 0{
			gains += change
		} else {
			losses += -change
		}
	}

	avgGain := gains / float64(period)
	avgLoss := gains / float64(period)

	if avgLoss == 0{
		return 100.0
	}

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))
	return rsi
}

func ATR(highs, lows, closes []float64, period int) float64 {
	if len(highs) < period+1 || len(lows) < period+1 || len(closes) < period+1{
		return 0
	}

	trueRanges := make([]float64, 0)

	for i := 1; i < len(closes); i++{
		highLow:= highs[i] - lows[i]
		highClose :=  math.Abs(highs[i]- closes[i-1])
		lowClose := math.Abs(lows[i]- closes[i-1])

		tr := math.Max(highLow, math.Max(highClose, lowClose))
		trueRanges = append(trueRanges, tr)

	}
		return SMA(trueRanges, period)
}

func MACD(prices []float64, fastPeriod, slowPeriod, signalPeriod int) (macd, signal, histogram float64)  {
	if len(prices) < slowPeriod{
		return 0, 0, 0
	}
	
	fastEMA := EMA(prices, fastPeriod)
	slowEMA := EMA(prices, slowPeriod)
	macd = fastEMA - slowEMA

	signal = macd * 0.9
	histogram = macd - signal

	return macd, signal, histogram
}

func BollingerBands(prices []float64, period int, stdDev float64) (middle, upper, lower float64)  {
	if len(prices) < period{
		return 0, 0, 0
	}


	middle = SMA(prices, period)

	variance := 0.0
	for i :=len(prices) - period; i < len(prices); i++{
		diff := prices[i] - middle
		variance += diff * diff
	}
	stdDeviation := math.Sqrt(variance / float64(period))

	upper = middle + (stdDeviation * stdDev)
	lower = middle - (stdDeviation * stdDev)

	return middle, upper, lower
}

func StdDev(prices []float64) float64  {
	if len(prices) == 0 {
		return 0
	}

	mean := 0.0
	for _, p := range prices{
		mean += p
	}
	mean /= float64(len(prices))

	variance := 0.0
	for _, p := range prices{
		diff := p - mean
		variance += diff * diff
	}
	variance /= float64(len(prices))

	return math.Sqrt(variance)
}

func PercentChange(oldPrice, newPrice float64) float64 {
	if oldPrice == 0 {
		return 0
	}
	return ((newPrice - oldPrice) / oldPrice) * 100
}


func Volatility(prices []float64, period int) float64 {
	if len(prices) < period {
		return 0
	}

	returns := make([]float64, 0)
	for i := len(prices) - period; i < len(prices)-1; i++ {
		ret := math.Log(prices[i+1] / prices[i])
		returns = append(returns, ret)
	}

	return StdDev(returns) * math.Sqrt(252) * 100
}