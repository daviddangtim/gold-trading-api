package risk

import (
	"errors"
	"gold-bot/indicators"
	"gold-bot/models"
	"sync"
	"time"
)

// RiskManager handles position sizing, stop losses, and risk limits
type RiskManager struct {
	AccountBalance     float64
	RiskPerTrade       float64 // Percentage of account to risk per trade (e.g., 0.02 = 2%)
	MaxDrawdownPercent float64 // Maximum allowed drawdown before stopping (e.g., 0.10 = 10%)
	MaxPositionSize    float64 // Maximum position size in ounces
	MaxDailyLoss       float64 // Maximum loss allowed per day
	MaxDailyTrades     int     // Maximum number of trades per day
	
	DailyLoss          float64
	DailyTradesCount   int
	PeakBalance        float64
	CurrentDrawdown    float64
	LastResetDate      time.Time
	
	mu sync.RWMutex
}

// NewRiskManager creates a new risk manager with sensible defaults
func NewRiskManager(initialBalance float64) *RiskManager {
	return &RiskManager{
		AccountBalance:     initialBalance,
		RiskPerTrade:       0.02,  // 2% risk per trade
		MaxDrawdownPercent: 0.10,  // 10% max drawdown
		MaxPositionSize:    10.0,  // 10 ounces max
		MaxDailyLoss:       initialBalance * 0.05, // 5% of account per day
		MaxDailyTrades:     10,
		PeakBalance:        initialBalance,
		LastResetDate:      time.Now(),
	}
}

// CalculatePositionSize determines how many ounces to trade based on risk
func (rm *RiskManager) CalculatePositionSize(price, stopLoss float64) (float64, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if price <= 0 || stopLoss <= 0 {
		return 0, errors.New("invalid price or stop loss")
	}

	// Calculate risk amount in dollars
	riskAmount := rm.AccountBalance * rm.RiskPerTrade
	
	// Calculate position size based on distance to stop loss
	priceRisk := price - stopLoss
	if priceRisk <= 0 {
		return 0, errors.New("stop loss must be below entry price")
	}
	
	positionSize := riskAmount / priceRisk
	
	// Cap at maximum position size
	if positionSize > rm.MaxPositionSize {
		positionSize = rm.MaxPositionSize
	}
	
	return positionSize, nil
}

// CalculateStopLoss calculates stop loss based on ATR
func (rm *RiskManager) CalculateStopLoss(price float64, atr float64, multiplier float64) float64 {
	// Stop loss = Current Price - (ATR * Multiplier)
	// Common multipliers: 1.5 - 3.0
	return price - (atr * multiplier)
}

// CalculateTakeProfit calculates take profit level
func (rm *RiskManager) CalculateTakeProfit(entryPrice, stopLoss float64, riskRewardRatio float64) float64 {
	// Risk-reward ratio: e.g., 2:1 means profit target is 2x the risk
	risk := entryPrice - stopLoss
	return entryPrice + (risk * riskRewardRatio)
}

// ShouldAllowTrade checks if a new trade is allowed based on risk limits
func (rm *RiskManager) ShouldAllowTrade() (bool, string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Reset daily counters if new day
	if time.Now().Day() != rm.LastResetDate.Day() {
		rm.DailyLoss = 0
		rm.DailyTradesCount = 0
		rm.LastResetDate = time.Now()
	}

	// Check max drawdown
	if rm.CurrentDrawdown >= rm.MaxDrawdownPercent {
		return false, "Maximum drawdown exceeded - trading halted"
	}

	// Check daily loss limit
	if rm.DailyLoss >= rm.MaxDailyLoss {
		return false, "Daily loss limit reached"
	}

	// Check daily trade limit
	if rm.DailyTradesCount >= rm.MaxDailyTrades {
		return false, "Daily trade limit reached"
	}

	return true, ""
}

// RecordTrade updates risk metrics after a trade
func (rm *RiskManager) RecordTrade(profitLoss float64) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.AccountBalance += profitLoss
	rm.DailyTradesCount++

	if profitLoss < 0 {
		rm.DailyLoss += -profitLoss
	}

	// Update peak balance and drawdown
	if rm.AccountBalance > rm.PeakBalance {
		rm.PeakBalance = rm.AccountBalance
		rm.CurrentDrawdown = 0
	} else {
		rm.CurrentDrawdown = (rm.PeakBalance - rm.AccountBalance) / rm.PeakBalance
	}
}

// ShouldStopLoss checks if stop loss should be triggered
func (rm *RiskManager) ShouldStopLoss(entryPrice, currentPrice, stopLoss float64, position models.Position) bool {
	if position == models.PositionLong {
		return currentPrice <= stopLoss
	} else if position == models.PositionShort {
		return currentPrice >= stopLoss
	}
	return false
}

// ShouldTakeProfit checks if take profit should be triggered
func (rm *RiskManager) ShouldTakeProfit(entryPrice, currentPrice, takeProfit float64, position models.Position) bool {
	if position == models.PositionLong {
		return currentPrice >= takeProfit
	} else if position == models.PositionShort {
		return currentPrice <= takeProfit
	}
	return false
}

// CalculateTrailingStop calculates trailing stop level
func (rm *RiskManager) CalculateTrailingStop(entryPrice, currentPrice, atr float64, position models.Position) float64 {
	// Trailing stop moves up with price but never down
	trailDistance := atr * 2.0
	
	if position == models.PositionLong {
		return currentPrice - trailDistance
	} else if position == models.PositionShort {
		return currentPrice + trailDistance
	}
	return 0
}

// GetRiskMetrics returns current risk metrics
func (rm *RiskManager) GetRiskMetrics() map[string]interface{} {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return map[string]interface{}{
		"account_balance":      rm.AccountBalance,
		"peak_balance":         rm.PeakBalance,
		"current_drawdown":     rm.CurrentDrawdown * 100,
		"daily_loss":           rm.DailyLoss,
		"daily_trades_count":   rm.DailyTradesCount,
		"max_daily_trades":     rm.MaxDailyTrades,
		"risk_per_trade":       rm.RiskPerTrade * 100,
		"max_drawdown_percent": rm.MaxDrawdownPercent * 100,
	}
}

// UpdateAccountBalance updates the account balance (for external updates)
func (rm *RiskManager) UpdateAccountBalance(newBalance float64) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	rm.AccountBalance = newBalance
	
	if newBalance > rm.PeakBalance {
		rm.PeakBalance = newBalance
	}
}

// CalculateVolatilityBasedStop adjusts stop loss based on current volatility
func (rm *RiskManager) CalculateVolatilityBasedStop(prices []float64, currentPrice float64) float64 {
	if len(prices) < 20 {
		// Default to 2% stop if insufficient data
		return currentPrice * 0.98
	}

	volatility := indicators.Volatility(prices, 20)
	
	// Wider stops in high volatility, tighter in low volatility
	stopPercent := 0.01 + (volatility / 100 * 0.02) // 1-3% range
	
	return currentPrice * (1 - stopPercent)
}