package main

import (
	"gold-bot/datafeed"
	"gold-bot/engine"
	"gold-bot/models"
	"gold-bot/risk"
	"gold-bot/strategy"
	"testing"
	"time"
)

// MockDataFeed simulates price data without API calls
type MockDataFeed struct {
	prices []float64
	index  int
}

func NewMockDataFeed(prices []float64) *MockDataFeed {
	return &MockDataFeed{prices: prices, index: 0}
}

func (m *MockDataFeed) GetName() string {
	return "Mock"
}

func (m *MockDataFeed) FetchPrice() (*models.PricePoint, error) {
	if m.index >= len(m.prices) {
		m.index = 0 // Loop back
	}

	price := m.prices[m.index]
	m.index++

	return &models.PricePoint{
		Price:     price,
		Timestamp: time.Now(),
		High:      price + 1,
		Low:       price - 1,
	}, nil
}

// TestTradingEngineWithMockData tests the full trading flow
func TestTradingEngineWithMockData(t *testing.T) {
	// Setup
	initialBalance := 10000.0
	trader := models.NewTrader(initialBalance)
	riskManager := risk.NewRiskManager(initialBalance)

	// Create mock price data (uptrend then downtrend)
	mockPrices := []float64{
		2000, 2010, 2020, 2030, 2040, 2050, // Uptrend
		2045, 2040, 2035, 2030, 2025, 2020, // Downtrend
	}

	mockFeed := NewMockDataFeed(mockPrices)
	fetcher := datafeed.NewPriceFetcher(mockFeed)
	priceStream := datafeed.NewPriceStream(fetcher, 100*time.Millisecond, 100)

	// Use simple momentum strategy
	strat := strategy.NewMomentumStrategy(0.3)

	tradingEngine := engine.NewTradingEngine(trader, riskManager, strat, priceStream)

	// Track trades
	tradesOpened := 0
	tradesClosed := 0

	tradingEngine.OnTradeOpen = func(trade *models.Trade) {
		tradesOpened++
		t.Logf("✅ Trade opened at %.2f", trade.EntryPrice)
	}

	tradingEngine.OnTradeClose = func(trade *models.Trade) {
		tradesClosed++
		t.Logf("✅ Trade closed at %.2f, P&L: %.2f", trade.ExitPrice, trade.ProfitLoss)
	}

	tradingEngine.OnPriceUpdate = func(price *models.PricePoint) {
		t.Logf("📊 Price update: %.2f", price.Price)
	}

	// Run for 2 seconds
	tradingEngine.Start()
	time.Sleep(2 * time.Second)
	tradingEngine.Stop()

	// Verify trades happened
	if tradesOpened == 0 {
		t.Log("⚠️  No trades opened (this might be OK depending on strategy)")
	}

	t.Logf("📈 Trades opened: %d, closed: %d", tradesOpened, tradesClosed)
	t.Logf("💰 Final balance: %.2f", trader.AccountBalance)

	// Verify trader state is valid
	state := trader.GetState()
	if state["account_balance"].(float64) < 0 {
		t.Error("Account balance should never be negative")
	}
}

// TestStrategySignals tests that strategies generate correct signals
func TestStrategySignals(t *testing.T) {
	t.Run("Momentum Strategy - Uptrend", func(t *testing.T) {
		// Create uptrend data
		uptrendPrices := []float64{100, 102, 104, 106, 108, 110, 112, 114, 116, 118, 120}
		history := models.NewPriceHistory(100)

		for _, price := range uptrendPrices {
			history.Add(models.PricePoint{Price: price, Timestamp: time.Now()})
		}

		// Test momentum strategy
		momentum := strategy.NewMomentumStrategy(1.0)
		signal := momentum.Evaluate(history)

		if signal.Signal != strategy.SignalBuy && signal.Signal != strategy.SignalHold {
			t.Errorf("Expected BUY or HOLD signal in uptrend, got %s", signal.Signal)
		}

		t.Logf("Signal: %s, Strength: %.2f, Reason: %s",
			signal.Signal, signal.Strength, signal.Reason)
	})

	t.Run("RSI Strategy - Oversold", func(t *testing.T) {
		// Create downtrend to get oversold RSI
		prices := []float64{120, 118, 116, 114, 112, 110, 108, 106, 104, 102, 100, 98, 96, 94, 92}
		history := models.NewPriceHistory(100)

		for _, price := range prices {
			history.Add(models.PricePoint{Price: price, Timestamp: time.Now()})
		}

		rsi := strategy.NewRSIStrategy(14, 30, 70)
		signal := rsi.Evaluate(history)

		t.Logf("RSI Signal: %s, Strength: %.2f, Reason: %s",
			signal.Signal, signal.Strength, signal.Reason)
	})

	t.Run("Multi-Strategy Engine", func(t *testing.T) {
		prices := []float64{100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110}
		history := models.NewPriceHistory(100)

		for _, price := range prices {
			history.Add(models.PricePoint{Price: price, Timestamp: time.Now()})
		}

		strategies := []strategy.Strategy{
			strategy.NewMomentumStrategy(0.5),
			strategy.NewRSIStrategy(14, 30, 70),
		}

		weights := map[string]float64{
			"Momentum": 1.0,
			"RSI":      1.0,
		}

		multiStrat := strategy.NewMultiStrategyEngine(strategies, weights)
		signal := multiStrat.Evaluate(history)

		t.Logf("Multi-Strategy Signal: %s, Strength: %.2f, Reason: %s",
			signal.Signal, signal.Strength, signal.Reason)
	})
}

// TestRiskManagement tests that risk limits are enforced
func TestRiskManagement(t *testing.T) {
	t.Run("Daily Loss Limit", func(t *testing.T) {
		rm := risk.NewRiskManager(10000)
		rm.MaxDailyLoss = 100.0

		// Simulate losses
		rm.RecordTrade(-50) // First loss
		rm.RecordTrade(-60) // Second loss (total: -110)

		// Should block further trades
		allowed, reason := rm.ShouldAllowTrade()
		if allowed {
			t.Error("Expected trade to be blocked after exceeding daily loss limit")
		}

		t.Logf("✅ Trade blocked: %s", reason)
	})

	t.Run("Position Sizing", func(t *testing.T) {
		rm := risk.NewRiskManager(10000)
		rm.RiskPerTrade = 0.02 // 2% risk

		entryPrice := 2000.0
		stopLoss := 1980.0 // $20 risk per ounce

		positionSize, err := rm.CalculatePositionSize(entryPrice, stopLoss)
		if err != nil {
			t.Fatalf("Error calculating position size: %v", err)
		}

		expectedRisk := 10000 * 0.02 // $200
		expectedSize := expectedRisk / (entryPrice - stopLoss)

		t.Logf("Position size: %.2f ounces (expected: %.2f)", positionSize, expectedSize)

		if positionSize <= 0 {
			t.Error("Position size should be positive")
		}
	})

	t.Run("Stop Loss Calculation", func(t *testing.T) {
		rm := risk.NewRiskManager(10000)

		currentPrice := 2000.0
		atr := 10.0
		multiplier := 2.0

		stopLoss := rm.CalculateStopLoss(currentPrice, atr, multiplier)
		expectedStop := currentPrice - (atr * multiplier)

		if stopLoss != expectedStop {
			t.Errorf("Stop loss = %.2f, want %.2f", stopLoss, expectedStop)
		}

		t.Logf("Stop loss at %.2f for price %.2f (ATR: %.2f)", stopLoss, currentPrice, atr)
	})

	t.Run("Max Drawdown Protection", func(t *testing.T) {
		rm := risk.NewRiskManager(10000)
		rm.MaxDrawdownPercent = 0.10 // 10%

		// Simulate 11% loss
		rm.RecordTrade(-1100)

		allowed, reason := rm.ShouldAllowTrade()
		if allowed {
			t.Error("Expected trade to be blocked after max drawdown exceeded")
		}

		t.Logf("✅ Trade blocked: %s", reason)
		t.Logf("Current drawdown: %.2f%%", rm.CurrentDrawdown*100)
	})
}

// TestPriceHistory tests the price history data structure
func TestPriceHistory(t *testing.T) {
	t.Run("Add and Retrieve", func(t *testing.T) {
		history := models.NewPriceHistory(10)

		// Add prices
		for i := 0; i < 15; i++ {
			history.Add(models.PricePoint{
				Price:     float64(2000 + i),
				Timestamp: time.Now(),
			})
		}

		// Should only keep last 10
		if history.Size() != 10 {
			t.Errorf("History size = %d, want 10", history.Size())
		}

		// Get last 5 prices
		last5 := history.GetPrices(5)
		if len(last5) != 5 {
			t.Errorf("Got %d prices, want 5", len(last5))
		}

		t.Logf("Last 5 prices: %v", last5)
	})

	t.Run("Latest Price", func(t *testing.T) {
		history := models.NewPriceHistory(10)
		history.Add(models.PricePoint{Price: 2050, Timestamp: time.Now()})

		latest, ok := history.Latest()
		if !ok {
			t.Error("Expected to get latest price")
		}

		if latest.Price != 2050 {
			t.Errorf("Latest price = %.2f, want 2050", latest.Price)
		}
	})
}

// TestTraderOperations tests trader state management
func TestTraderOperations(t *testing.T) {
	t.Run("Open and Close Position", func(t *testing.T) {
		trader := models.NewTrader(10000)

		// Open long position
		trader.OpenPosition(models.PositionLong, 2000)

		if !trader.IsInPosition() {
			t.Error("Expected trader to be in position")
		}

		// Update price
		trader.UpdatePrice(2050)

		state := trader.GetState()
		currentPnL := state["current_pnl"].(float64)

		if currentPnL != 50 {
			t.Errorf("Current P&L = %.2f, want 50", currentPnL)
		}

		// Close position
		trade := trader.ClosePosition(2050)

		if trade == nil {
			t.Fatal("Expected trade to be returned")
		}

		if trade.ProfitLoss != 50 {
			t.Errorf("Trade P&L = %.2f, want 50", trade.ProfitLoss)
		}

		if trader.IsInPosition() {
			t.Error("Expected trader to not be in position")
		}

		t.Logf("✅ Trade completed: Entry=%.2f, Exit=%.2f, P&L=%.2f",
			trade.EntryPrice, trade.ExitPrice, trade.ProfitLoss)
	})

	t.Run("Prevent Double Position", func(t *testing.T) {
		trader := models.NewTrader(10000)

		trader.OpenPosition(models.PositionLong, 2000)
		trader.OpenPosition(models.PositionLong, 2010) // Should be ignored

		state := trader.GetState()
		entryPrice := state["entry_price"].(float64)

		if entryPrice != 2000 {
			t.Error("Second position open should have been ignored")
		}
	})
}

// BenchmarkTradingLoop benchmarks the main trading loop
func BenchmarkTradingLoop(b *testing.B) {
	trader := models.NewTrader(10000)
	riskManager := risk.NewRiskManager(10000)

	mockPrices := []float64{2000, 2010, 2020, 2030, 2040, 2050}
	mockFeed := NewMockDataFeed(mockPrices)
	fetcher := datafeed.NewPriceFetcher(mockFeed)
	priceStream := datafeed.NewPriceStream(fetcher, 10*time.Millisecond, 100)

	strat := strategy.NewMomentumStrategy(0.5)
	tradingEngine := engine.NewTradingEngine(trader, riskManager, strat, priceStream)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tradingEngine.Start()
		time.Sleep(50 * time.Millisecond)
		tradingEngine.Stop()
	}
}