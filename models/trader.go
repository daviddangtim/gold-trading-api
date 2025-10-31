package models

import (
	"sync"
	"time"
)

type Position string

const (
	PositionNone  Position = "NONE"
	PositionLong  Position = "LONG"
	PositionShort Position = "SHORT"
)

type Trade struct {
	ID            string    `json:"id"`
	EntryPrice    float64   `json:"entry_price"`
	ExitPrice     float64   `json:"exit_price"`
	Quantity      float64   `json:"quantity"`
	Position      Position  `json:"position"`
	EntryTime     time.Time `json:"entry_time"`
	ExitTime      time.Time `json:"exit_time"`
	ProfitLoss    float64   `json:"profit_loss"`
	ProfitPercent float64   `json:"profit_percent"`
}

type Trader struct {
	LastPrice      float64   `json:"last_price"`
	Position       Position  `json:"position"`
	Timestamp      int64     `json:"timestamp"`
	EntryPrice     float64   `json:"entry_price"`
	EntryTime      time.Time `json:"entry_time"`
	CurrentPnL     float64   `json:"current_pnl"`
	TotalPnL       float64   `json:"total_pnl"`
	TradeCount     int       `json:"trade_count"`
	WinCount       int       `json:"win_count"`
	LossCount      int       `json:"loss_count"`
	AccountBalance float64   `json:"account_balance"`
	mu             sync.RWMutex
}

func NewTrader(initialBalance float64) *Trader {
	return &Trader{
		Position:       PositionNone,
		AccountBalance: initialBalance,
	}
}

func (t *Trader) UpdatePrice(price float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.LastPrice = price
	t.Timestamp = time.Now().Unix()

	if t.Position == PositionLong {
		t.CurrentPnL = price - t.EntryPrice
	} else if t.Position == PositionShort {
		t.CurrentPnL = t.EntryPrice - price
	} else {
		t.CurrentPnL = 0
	}
}

func (t *Trader) ClosePosition(price float64) *Trade {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.Position == PositionNone {
		return nil
	}

	var profitLoss float64
	if t.Position == PositionLong {
		profitLoss = price - t.EntryPrice
	} else if t.Position == PositionShort {
		profitLoss = t.EntryPrice - price
	}

	profitPercent := (profitLoss / t.EntryPrice) * 100

	trade := &Trade{
		EntryPrice:    t.EntryPrice,
		ExitPrice:     price,
		Position:      t.Position,
		EntryTime:     t.EntryTime,
		ExitTime:      time.Now(),
		ProfitLoss:    profitLoss,
		ProfitPercent: profitPercent,
	}

	t.TotalPnL += profitLoss
	t.AccountBalance += profitLoss
	t.TradeCount++
	if profitLoss > 0 {
		t.WinCount++
	} else {
		t.LossCount++
	}

	t.Position = PositionNone
	t.EntryPrice = 0
	t.CurrentPnL = 0

	return trade
}

func (t *Trader) GetState() map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	winRate := 0.0
	if t.TradeCount > 0 {
		winRate = float64(t.WinCount) / float64(t.TradeCount*100)
	}

	return map[string]interface{}{
		"last_price":      t.LastPrice,
		"position":        t.Position,
		"entry_price":     t.EntryPrice,
		"current_pnl":     t.CurrentPnL,
		"total_pnl":       t.TotalPnL,
		"account_balance": t.AccountBalance,
		"trade_count":     t.TradeCount,
		"win_count":       t.WinCount,
		"loss_count":      t.LossCount,
		"win_rate":        winRate,
		"timestamp":       t.Timestamp,
	}
}

func (t *Trader) IsInPosition() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Position != PositionNone
}

func (t *Trader) GetPosition() Position {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Position
}
