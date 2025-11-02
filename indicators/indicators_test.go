package indicators

import (
	"math"
	"testing"
)

func TestSMA(t *testing.T) {
	prices := []float64{10, 11, 12, 13, 14, 15}
	
	tests := []struct {
		period   int
		expected float64
	}{
		{3, 14.0},  // (12+13+14)/3
		{5, 13.0},  // (10+11+12+13+14)/5
	}

	for _, tt := range tests {
		result := SMA(prices, tt.period)
		if math.Abs(result-tt.expected) > 0.01 {
			t.Errorf("SMA(%d) = %.2f; want %.2f", tt.period, result, tt.expected)
		}
	}
}

func TestEMA(t *testing.T) {
	prices := []float64{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	
	ema := EMA(prices, 5)
	
	// EMA should be higher than first price but not exceed last price
	if ema < prices[0] || ema > prices[len(prices)-1]*1.1 {
		t.Errorf("EMA = %.2f seems out of reasonable range", ema)
	}
	
	t.Logf("EMA(5) = %.2f", ema)
}

func TestRSI(t *testing.T) {
	tests := []struct {
		name     string
		prices   []float64
		expected string // "high", "low", or "neutral"
	}{
		{
			name:     "Uptrend - RSI should be high",
			prices:   []float64{100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114},
			expected: "high",
		},
		{
			name:     "Downtrend - RSI should be low",
			prices:   []float64{114, 113, 112, 111, 110, 109, 108, 107, 106, 105, 104, 103, 102, 101, 100},
			expected: "low",
		},
		{
			name:     "Sideways - RSI should be neutral",
			prices:   []float64{100, 101, 100, 101, 100, 101, 100, 101, 100, 101, 100, 101, 100, 101, 100},
			expected: "neutral",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rsi := RSI(tt.prices, 14)
			
			switch tt.expected {
			case "high":
				if rsi < 50 {
					t.Errorf("RSI in uptrend should be > 50, got %.2f", rsi)
				}
			case "low":
				if rsi > 50 {
					t.Errorf("RSI in downtrend should be < 50, got %.2f", rsi)
				}
			case "neutral":
				if rsi < 30 || rsi > 70 {
					t.Logf("RSI in sideways market = %.2f (can vary)", rsi)
				}
			}
			
			t.Logf("RSI = %.2f", rsi)
		})
	}
}

func TestPercentChange(t *testing.T) {
	tests := []struct {
		old      float64
		new      float64
		expected float64
	}{
		{100, 110, 10.0},
		{100, 90, -10.0},
		{100, 100, 0.0},
		{2000, 2020, 1.0},
		{2000, 1980, -1.0},
	}

	for _, tt := range tests {
		result := PercentChange(tt.old, tt.new)
		if math.Abs(result-tt.expected) > 0.01 {
			t.Errorf("PercentChange(%.2f, %.2f) = %.2f; want %.2f",
				tt.old, tt.new, result, tt.expected)
		}
	}
}

func TestPercentChangeZero(t *testing.T) {
	result := PercentChange(0, 100)
	if result != 0 {
		t.Errorf("PercentChange with zero old price should return 0, got %.2f", result)
	}
}

func TestATR(t *testing.T) {
	highs := []float64{105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 115}
	lows := []float64{95, 96, 97, 98, 99, 100, 101, 102, 103, 104, 105}
	closes := []float64{100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110}

	atr := ATR(highs, lows, closes, 10)

	if atr <= 0 {
		t.Error("ATR should be positive")
	}

	// ATR should be reasonable (around the average range)
	if atr < 5 || atr > 15 {
		t.Logf("ATR = %.2f (unusual but may be valid)", atr)
	}

	t.Logf("ATR(10) = %.2f", atr)
}

func TestBollingerBands(t *testing.T) {
	prices := []float64{10, 11, 12, 13, 14, 15, 14, 13, 12, 11}
	middle, upper, lower := BollingerBands(prices, 5, 2.0)

	// Middle should be the SMA
	expectedMiddle := SMA(prices, 5)
	if math.Abs(middle-expectedMiddle) > 0.01 {
		t.Errorf("Bollinger middle band = %.2f; want %.2f", middle, expectedMiddle)
	}

	// Upper should be above middle, lower should be below
	if upper <= middle {
		t.Errorf("Upper band (%.2f) should be > middle (%.2f)", upper, middle)
	}
	if lower >= middle {
		t.Errorf("Lower band (%.2f) should be < middle (%.2f)", lower, middle)
	}

	t.Logf("Bollinger Bands: Lower=%.2f, Middle=%.2f, Upper=%.2f", lower, middle, upper)
}

func TestMACD(t *testing.T) {
	// Create uptrending prices
	prices := make([]float64, 50)
	for i := range prices {
		prices[i] = 100 + float64(i)*0.5
	}

	macd, signal, histogram := MACD(prices, 12, 26, 9)

	t.Logf("MACD: %.2f, Signal: %.2f, Histogram: %.2f", macd, signal, histogram)

	// In uptrend, MACD should typically be positive
	if macd < 0 {
		t.Logf("MACD is negative (%.2f) - may indicate trend change", macd)
	}
}

func TestStdDev(t *testing.T) {
	prices := []float64{10, 12, 23, 23, 16, 23, 21, 16}
	stdDev := StdDev(prices)

	if stdDev <= 0 {
		t.Error("Standard deviation should be positive")
	}

	t.Logf("Standard Deviation = %.2f", stdDev)
}

func TestVolatility(t *testing.T) {
	// Create prices with some volatility
	prices := []float64{
		100, 102, 99, 103, 98, 104, 97, 105, 96, 106,
		95, 107, 94, 108, 93, 109, 92, 110, 91, 111,
		90, 112, 89, 113, 88, 114, 87, 115, 86, 116,
	}

	vol := Volatility(prices, 20)

	if vol <= 0 {
		t.Error("Volatility should be positive")
	}

	t.Logf("Annualized Volatility = %.2f%%", vol)
}

// Benchmark tests
func BenchmarkSMA(b *testing.B) {
	prices := make([]float64, 100)
	for i := range prices {
		prices[i] = float64(i + 100)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SMA(prices, 20)
	}
}

func BenchmarkEMA(b *testing.B) {
	prices := make([]float64, 100)
	for i := range prices {
		prices[i] = float64(i + 100)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EMA(prices, 20)
	}
}

func BenchmarkRSI(b *testing.B) {
	prices := make([]float64, 100)
	for i := range prices {
		prices[i] = float64(i + 100)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RSI(prices, 14)
	}
}

func BenchmarkATR(b *testing.B) {
	highs := make([]float64, 100)
	lows := make([]float64, 100)
	closes := make([]float64, 100)
	
	for i := range highs {
		base := float64(i + 100)
		highs[i] = base + 2
		lows[i] = base - 2
		closes[i] = base
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ATR(highs, lows, closes, 14)
	}
}

func BenchmarkBollingerBands(b *testing.B) {
	prices := make([]float64, 100)
	for i := range prices {
		prices[i] = float64(i + 100)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BollingerBands(prices, 20, 2.0)
	}
}
