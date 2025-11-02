package engine

import (
	"fmt"
	"gold-bot/datafeed"
	"gold-bot/indicators"
	"gold-bot/models"
	"gold-bot/risk"
	"gold-bot/strategy"
	"log"
	"time"
)

// TradingEngine orchestrates the entire trading system
type TradingEngine struct {
	Trader       *models.Trader
	RiskManager  *risk.RiskManager
	Strategy     strategy.Strategy
	PriceStream  *datafeed.PriceStream
	
	StopLoss     float64
	TakeProfit   float64
	TrailingStop bool
	
	OnTradeOpen  func(*models.Trade)
	OnTradeClose func(*models.Trade)
	OnPriceUpdate func(*models.PricePoint)
}

// NewTradingEngine creates a new trading engine
func NewTradingEngine(
	trader *models.Trader,
	riskManager *risk.RiskManager,
	strat strategy.Strategy,
	priceStream *datafeed.PriceStream,
) *TradingEngine {
	return &TradingEngine{
		Trader:       trader,
		RiskManager:  riskManager,
		Strategy:     strat,
		PriceStream:  priceStream,
		TrailingStop: true,
	}
}

// Start begins the trading engine
func (te *TradingEngine) Start() {
	log.Println("🚀 Trading engine started")
	
	// Start price stream
	te.PriceStream.Start()

	// Main trading loop
	go func() {
		for {
			select {
			case pricePoint := <-te.PriceStream.PriceChan():
				te.processPriceTick(pricePoint)
			case err := <-te.PriceStream.ErrorChan():
				log.Printf("❌ Price stream error: %v", err)
			}
		}
	}()
}

// Stop stops the trading engine
func (te *TradingEngine) Stop() {
	te.PriceStream.Stop()
	
	// Close any open positions
	if te.Trader.IsInPosition() {
		latest, _ := te.PriceStream.GetLatestPrice()
		te.closePosition(latest.Price, "Engine shutdown")
	}
	
	log.Println("🛑 Trading engine stopped")
}

// processPriceTick handles each price update
func (te *TradingEngine) processPriceTick(pricePoint *models.PricePoint) {
	// Update trader with new price
	te.Trader.UpdatePrice(pricePoint.Price)

	// Notify listeners
	if te.OnPriceUpdate != nil {
		te.OnPriceUpdate(pricePoint)
	}

	// Check if we're in a position
	if te.Trader.IsInPosition() {
		te.manageOpenPosition(pricePoint.Price)
	} else {
		te.evaluateEntry(pricePoint.Price)
	}
}

// evaluateEntry checks if we should enter a new position
func (te *TradingEngine) evaluateEntry(currentPrice float64) {
	// Check risk limits
	allowed, reason := te.RiskManager.ShouldAllowTrade()
	if !allowed {
		log.Printf("⛔ Trade blocked: %s", reason)
		return
	}

	// Get signal from strategy
	signal := te.Strategy.Evaluate(te.PriceStream.GetHistory())
	
	if signal.Signal == strategy.SignalBuy {
		te.openLongPosition(currentPrice, signal)
	} else if signal.Signal == strategy.SignalSell {
		// For now, we only go long. Short selling would go here
		log.Printf("📉 Sell signal received but short selling not implemented")
	}
}

// openLongPosition opens a long position
func (te *TradingEngine) openLongPosition(price float64, signal strategy.TradeSignal) {
	// Calculate stop loss using ATR
	prices := te.PriceStream.GetHistory().GetPrices(20)
	if len(prices) < 20 {
		log.Println("⚠️  Insufficient data for ATR calculation")
		return
	}

	// Simple stop loss calculation
	atr := calculateSimpleATR(prices, 14)
	te.StopLoss = te.RiskManager.CalculateStopLoss(price, atr, 2.0)
	te.TakeProfit = te.RiskManager.CalculateTakeProfit(price, te.StopLoss, 2.0)

	// Open position
	te.Trader.OpenPosition(models.PositionLong, price)

	log.Printf("📈 LONG @ %.2f | SL: %.2f | TP: %.2f | Reason: %s",
		price, te.StopLoss, te.TakeProfit, signal.Reason)

	// Notify listeners
	if te.OnTradeOpen != nil {
		trade := &models.Trade{
			EntryPrice: price,
			Position:   models.PositionLong,
			EntryTime:  time.Now(),
		}
		te.OnTradeOpen(trade)
	}
}

// manageOpenPosition manages an open position
func (te *TradingEngine) manageOpenPosition(currentPrice float64) {
	position := te.Trader.GetPosition()
	state := te.Trader.GetState()
	entryPrice := state["entry_price"].(float64)

	// Check stop loss
	if te.RiskManager.ShouldStopLoss(entryPrice, currentPrice, te.StopLoss, position) {
		te.closePosition(currentPrice, "Stop loss hit")
		return
	}

	// Check take profit
	if te.RiskManager.ShouldTakeProfit(entryPrice, currentPrice, te.TakeProfit, position) {
		te.closePosition(currentPrice, "Take profit hit")
		return
	}

	// Update trailing stop if enabled
	if te.TrailingStop && position == models.PositionLong {
		prices := te.PriceStream.GetHistory().GetPrices(14)
		atr := calculateSimpleATR(prices, 14)
		newStop := te.RiskManager.CalculateTrailingStop(entryPrice, currentPrice, atr, position)
		
		// Only move stop up, never down
		if newStop > te.StopLoss {
			te.StopLoss = newStop
			log.Printf("📊 Trailing stop updated to %.2f", newStop)
		}
	}
}

// closePosition closes the current position
func (te *TradingEngine) closePosition(price float64, reason string) {
	trade := te.Trader.ClosePosition(price)
	if trade == nil {
		return
	}

	// Record with risk manager
	te.RiskManager.RecordTrade(trade.ProfitLoss)

	emoji := "💰"
	if trade.ProfitLoss < 0 {
		emoji = "📉"
	}

	log.Printf("%s CLOSE @ %.2f | P&L: %.2f (%.2f%%) | Reason: %s",
		emoji, price, trade.ProfitLoss, trade.ProfitPercent, reason)

	// Notify listeners
	if te.OnTradeClose != nil {
		te.OnTradeClose(trade)
	}
}

// GetStatus returns current engine status
func (te *TradingEngine) GetStatus() map[string]interface{} {
	latestPrice, _ := te.PriceStream.GetLatestPrice()
	
	status := map[string]interface{}{
		"trader":       te.Trader.GetState(),
		"risk_metrics": te.RiskManager.GetRiskMetrics(),
		"strategy":     te.Strategy.GetName(),
		"latest_price": latestPrice,
	}

	if te.Trader.IsInPosition() {
		status["stop_loss"] = te.StopLoss
		status["take_profit"] = te.TakeProfit
	}

	return status
}

// calculateSimpleATR is a helper to calculate ATR from price data
func calculateSimpleATR(prices []float64, period int) float64 {
	if len(prices) < period+1 {
		return 0
	}

	// For simplicity, use price ranges as proxy for true range
	ranges := make([]float64, 0)
	for i := 1; i < len(prices); i++ {
		rangeVal := prices[i] - prices[i-1]
		if rangeVal < 0 {
			rangeVal = -rangeVal
		}
		ranges = append(ranges, rangeVal)
	}

	return indicators.SMA(ranges, period)
}

// UpdateStrategy changes the active trading strategy
func (te *TradingEngine) UpdateStrategy(newStrategy strategy.Strategy) {
	// Close any open position before switching strategies
	if te.Trader.IsInPosition() {
		latest, _ := te.PriceStream.GetLatestPrice()
		te.closePosition(latest.Price, "Strategy change")
	}

	te.Strategy = newStrategy
	log.Printf("📋 Strategy updated to: %s", newStrategy.GetName())
}

// SetRiskParameters updates risk management parameters
func (te *TradingEngine) SetRiskParameters(riskPerTrade, maxDrawdown float64) {
	te.RiskManager.RiskPerTrade = riskPerTrade
	te.RiskManager.MaxDrawdownPercent = maxDrawdown
	log.Printf("⚙️  Risk parameters updated: Risk/Trade=%.2f%%, MaxDrawdown=%.2f%%",
		riskPerTrade*100, maxDrawdown*100)
}

// EnableTrailingStop enables or disables trailing stops
func (te *TradingEngine) EnableTrailingStop(enabled bool) {
	te.TrailingStop = enabled
	status := "disabled"
	if enabled {
		status = "enabled"
	}
	log.Printf("🎯 Trailing stop %s", status)
}

// ForceClosePosition manually closes the current position
func (te *TradingEngine) ForceClosePosition() error {
	if !te.Trader.IsInPosition() {
		return fmt.Errorf("no open position to close")
	}

	latest, err := te.PriceStream.GetLatestPrice()
	if err != nil {
		return err
	}

	te.closePosition(latest.Price, "Manual close")
	return nil
}
