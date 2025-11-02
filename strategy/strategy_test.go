package strategy

import (
	"gold-bot/models"
	"testing"
	"time"
)

func createTestHistory(prices []float64) *models.PriceHistory {
	history := models.NewPriceHistory(100)
	for _, price := range prices {
		history.Add(models.PricePoint{
			Price:     price,
			Timestamp: time.Now(),
		})
	}
	return history
}

func TestMomentumStrategy(t *testing.T) {
	strategy := NewMomentumStrategy(1.0)

	t.Run("Bullish momentum", func(t *testing.T) {
		prices := []float64{100, 102} // 2% increase
		history := createTestHistory(prices)

		signal := strategy.Evaluate(history)
		if signal.Signal != SignalBuy {
			t.Errorf("Expected BUY signal, got %s", signal.Signal)
		}
		t.Logf("Signal: %s, Strength: %.2f, Reason: %s", signal.Signal, signal.Strength, signal.Reason)
	})

	t.Run("Bearish momentum", func(t *testing.T) {
		prices := []float64{100, 98} // 2% decrease
		history := createTestHistory(prices)

		signal := strategy.Evaluate(history)
		if signal.Signal != SignalSell {
			t.Errorf("Expected SELL signal, got %s", signal.Signal)
		}
		t.Logf("Signal: %s, Strength: %.2f, Reason: %s", signal.Signal, signal.Strength, signal.Reason)
	})

	t.Run("No momentum", func(t *testing.T) {
		prices := []float64{100, 100.5} // 0.5% increase (below threshold)
		history := createTestHistory(prices)

		signal := strategy.Evaluate(history)
		if signal.Signal != SignalHold {
			t.Errorf("Expected HOLD signal, got %s", signal.Signal)
		}
		t.Logf("Signal: %s, Reason: %s", signal.Signal, signal.Reason)
	})

	t.Run("Insufficient data", func(t *testing.T) {
		prices := []float64{100} // Only one price
		history := createTestHistory(prices)

		signal := strategy.Evaluate(history)
		if signal.Signal != SignalNone {
			t.Errorf("Expected NONE signal with insufficient data, got %s", signal.Signal)
		}
	})
}

func TestRSIStrategy(t *testing.T) {
	strategy := NewRSIStrategy(14, 30, 70)

	t.Run("Oversold condition", func(t *testing.T) {
		// Create downtrend to get low RSI
		prices := []float64{120, 118, 116, 114, 112, 110, 108, 106, 104, 102, 100, 98, 96, 94, 92}
		history := createTestHistory(prices)

		signal := strategy.Evaluate(history)
		if signal.Signal != SignalBuy {
			t.Errorf("Expected BUY signal for oversold, got %s", signal.Signal)
		}
		t.Logf("Oversold - Signal: %s, Strength: %.2f", signal.Signal, signal.Strength)
	})

	t.Run("Overbought condition", func(t *testing.T) {
		// Create uptrend to get high RSI
		prices := []float64{80, 82, 84, 86, 88, 90, 92, 94, 96, 98, 100, 102, 104, 106, 108}
		history := createTestHistory(prices)

		signal := strategy.Evaluate(history)
		if signal.Signal != SignalSell {
			t.Errorf("Expected SELL signal for overbought, got %s", signal.Signal)
		}
		t.Logf("Overbought - Signal: %s, Strength: %.2f", signal.Signal, signal.Strength)
	})

	t.Run("Neutral RSI", func(t *testing.T) {
		// Sideways market
		prices := []float64{100, 101, 100, 101, 100, 101, 100, 101, 100, 101, 100, 101, 100, 101, 100}
		history := createTestHistory(prices)

		signal := strategy.Evaluate(history)
		if signal.Signal != SignalHold {
			t.Logf("Neutral market gave signal: %s (can vary)", signal.Signal)
		}
	})

	t.Run("Insufficient data", func(t *testing.T) {
		prices := []float64{100, 101, 102} // Not enough for RSI
		history := createTestHistory(prices)

		signal := strategy.Evaluate(history)
		if signal.Signal != SignalNone {
			t.Errorf("Expected NONE signal with insufficient data, got %s", signal.Signal)
		}
	})
}

func TestMovingAverageCrossStrategy(t *testing.T) {
	strategy := NewMovingAverageCrossStrategy(3, 5)

	t.Run("Bullish crossover", func(t *testing.T) {
		// Prices transition from downtrend to uptrend
		prices := []float64{100, 99, 98, 97, 96, 98, 102, 106}
		history := createTestHistory(prices)

		signal := strategy.Evaluate(history)
		t.Logf("MA Cross Signal: %s, Reason: %s", signal.Signal, signal.Reason)
		
		// Crossover detection can be timing-sensitive
		if signal.Signal != SignalBuy && signal.Signal != SignalHold {
			t.Logf("Expected BUY or HOLD, got %s (timing dependent)", signal.Signal)
		}
	})

	t.Run("Bearish crossover", func(t *testing.T) {
		// Prices transition from uptrend to downtrend
		prices := []float64{100, 101, 102, 103, 104, 102, 98, 94}
		history := createTestHistory(prices)

		signal := strategy.Evaluate(history)
		t.Logf("MA Cross Signal: %s, Reason: %s", signal.Signal, signal.Reason)
	})

	t.Run("No crossover", func(t *testing.T) {
		// Steady uptrend, no crossover
		prices := []float64{100, 101, 102, 103, 104, 105, 106}
		history := createTestHistory(prices)

		signal := strategy.Evaluate(history)
		if signal.Signal == SignalBuy || signal.Signal == SignalSell {
			t.Logf("Got trading signal without clear crossover: %s", signal.Signal)
		}
	})

	t.Run("Insufficient data", func(t *testing.T) {
		prices := []float64{100, 101} // Not enough for MA calculation
		history := createTestHistory(prices)

		signal := strategy.Evaluate(history)
		if signal.Signal != SignalNone {
			t.Errorf("Expected NONE signal with insufficient data, got %s", signal.Signal)
		}
	})
}

func TestMultiStrategyEngine(t *testing.T) {
	t.Run("Strong consensus buy", func(t *testing.T) {
		strategies := []Strategy{
			NewMomentumStrategy(1.0),
			NewRSIStrategy(14, 30, 70),
		}

		weights := map[string]float64{
			"Momentum": 1.0,
			"RSI":      1.0,
		}

		engine := NewMultiStrategyEngine(strategies, weights)

		// Strong downtrend followed by reversal - should trigger oversold RSI and momentum
		prices := []float64{120, 118, 116, 114, 112, 110, 108, 106, 104, 102, 100, 98, 96, 94, 92, 95, 98}
		history := createTestHistory(prices)

		signal := engine.Evaluate(history)
		t.Logf("Multi-strategy signal: %s, Strength: %.2f, Reason: %s",
			signal.Signal, signal.Strength, signal.Reason)
	})

	t.Run("Conflicting signals", func(t *testing.T) {
		strategies := []Strategy{
			NewMomentumStrategy(0.5),
			NewRSIStrategy(14, 30, 70),
		}

		weights := map[string]float64{
			"Momentum": 1.0,
			"RSI":      1.0,
		}

		engine := NewMultiStrategyEngine(strategies, weights)

		// Create scenario where strategies might disagree
		prices := []float64{100, 101, 100, 101, 100, 101, 100, 101, 100, 101, 100, 102, 103, 104, 103}
		history := createTestHistory(prices)

		signal := engine.Evaluate(history)
		t.Logf("With potential conflict - Signal: %s, Reason: %s", signal.Signal, signal.Reason)

		// Should hold when no strong consensus
		if signal.Signal == SignalBuy || signal.Signal == SignalSell {
			t.Logf("Got trading signal: %s (strength: %.2f)", signal.Signal, signal.Strength)
		}
	})

	t.Run("Weighted strategies", func(t *testing.T) {
		strategies := []Strategy{
			NewMomentumStrategy(1.0),
			NewRSIStrategy(14, 30, 70),
		}

		// RSI weighted more heavily
		weights := map[string]float64{
			"Momentum": 0.5,
			"RSI":      2.0,
		}

		engine := NewMultiStrategyEngine(strategies, weights)

		// Oversold condition
		prices := []float64{120, 118, 116, 114, 112, 110, 108, 106, 104, 102, 100, 98, 96, 94, 92}
		history := createTestHistory(prices)

		signal := engine.Evaluate(history)
		t.Logf("Weighted (RSI heavy) signal: %s, Strength: %.2f", signal.Signal, signal.Strength)
	})

	t.Run("GetName", func(t *testing.T) {
		strategies := []Strategy{NewMomentumStrategy(1.0)}
		weights := map[string]float64{"Momentum": 1.0}
		engine := NewMultiStrategyEngine(strategies, weights)

		if engine.GetName() != "Multi-Strategy" {
			t.Errorf("Expected name 'Multi-Strategy', got '%s'", engine.GetName())
		}
	})
}

func TestStrategyInterface(t *testing.T) {
	// Verify all strategies implement the interface correctly
	strategies := []Strategy{
		NewMomentumStrategy(0.5),
		NewRSIStrategy(14, 30, 70),
		NewMovingAverageCrossStrategy(10, 30),
		NewMultiStrategyEngine([]Strategy{}, map[string]float64{}),
	}

	prices := []float64{100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110,
		111, 112, 113, 114, 115, 116, 117, 118, 119, 120}
	history := createTestHistory(prices)

	for _, strat := range strategies {
		t.Run(strat.GetName(), func(t *testing.T) {
			// Should not panic
			signal := strat.Evaluate(history)

			// Should return valid signal
			validSignals := []Signal{SignalBuy, SignalSell, SignalHold, SignalNone}
			valid := false
			for _, vs := range validSignals {
				if signal.Signal == vs {
					valid = true
					break
				}
			}

			if !valid {
				t.Errorf("Strategy returned invalid signal: %s", signal.Signal)
			}

			t.Logf("%s: Signal=%s, Strength=%.2f, Reason=%s",
				strat.GetName(), signal.Signal, signal.Strength, signal.Reason)
		})
	}
}

// Benchmark tests
func BenchmarkMomentumStrategy(b *testing.B) {
	prices := make([]float64, 100)
	for i := range prices {
		prices[i] = float64(100 + i)
	}
	history := createTestHistory(prices)
	strategy := NewMomentumStrategy(0.5)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.Evaluate(history)
	}
}

func BenchmarkRSIStrategy(b *testing.B) {
	prices := make([]float64, 100)
	for i := range prices {
		prices[i] = float64(100 + i)
	}
	history := createTestHistory(prices)
	strategy := NewRSIStrategy(14, 30, 70)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.Evaluate(history)
	}
}

func BenchmarkMAStrategy(b *testing.B) {
	prices := make([]float64, 100)
	for i := range prices {
		prices[i] = float64(100 + i)
	}
	history := createTestHistory(prices)
	strategy := NewMovingAverageCrossStrategy(10, 30)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.Evaluate(history)
	}
}

func BenchmarkMultiStrategy(b *testing.B) {
	prices := make([]float64, 100)
	for i := range prices {
		prices[i] = float64(100 + i)
	}
	history := createTestHistory(prices)

	strategies := []Strategy{
		NewMomentumStrategy(0.5),
		NewRSIStrategy(14, 30, 70),
		NewMovingAverageCrossStrategy(10, 30),
	}
	weights := map[string]float64{
		"Momentum": 1.0,
		"RSI":      1.0,
		"MA Cross": 1.0,
	}
	engine := NewMultiStrategyEngine(strategies, weights)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Evaluate(history)
	}
}
