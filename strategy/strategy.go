package strategy

import (
	"gold-bot/indicators"
	"gold-bot/models"
)

// Signal represents a trading signal
type Signal string

const (
	SignalBuy    Signal = "BUY"
	SignalSell   Signal = "SELL"
	SignalHold   Signal = "HOLD"
	SignalNone   Signal = "NONE"
)

// SignalStrength represents confidence in a signal (0-100)
type SignalStrength float64

// TradeSignal contains the signal and its strength
type TradeSignal struct {
	Signal   Signal
	Strength SignalStrength
	Reason   string
	Price    float64
}

// Strategy interface for different trading strategies
type Strategy interface {
	Evaluate(history *models.PriceHistory) TradeSignal
	GetName() string
}

// MomentumStrategy - simple momentum-based strategy (your original logic)
type MomentumStrategy struct {
	Threshold float64 // Percentage threshold for entry
}

func NewMomentumStrategy(threshold float64) *MomentumStrategy {
	return &MomentumStrategy{Threshold: threshold}
}

func (s *MomentumStrategy) GetName() string {
	return "Momentum"
}

func (s *MomentumStrategy) Evaluate(history *models.PriceHistory) TradeSignal {
	if history.Size() < 2 {
		return TradeSignal{Signal: SignalNone, Reason: "Insufficient data"}
	}

	prices := history.GetPrices(2)
	change := indicators.PercentChange(prices[0], prices[1])

	if change > s.Threshold {
		return TradeSignal{
			Signal:   SignalBuy,
			Strength: SignalStrength(change * 10),
			Reason:   "Positive momentum",
			Price:    prices[1],
		}
	}

	if change < -s.Threshold {
		return TradeSignal{
			Signal:   SignalSell,
			Strength: SignalStrength(-change * 10),
			Reason:   "Negative momentum",
			Price:    prices[1],
		}
	}

	return TradeSignal{Signal: SignalHold, Reason: "No strong momentum"}
}

// RSIStrategy - mean reversion based on RSI
type RSIStrategy struct {
	Period         int
	OversoldLevel  float64 // e.g., 30
	OverboughtLevel float64 // e.g., 70
}

func NewRSIStrategy(period int, oversold, overbought float64) *RSIStrategy {
	return &RSIStrategy{
		Period:         period,
		OversoldLevel:  oversold,
		OverboughtLevel: overbought,
	}
}

func (s *RSIStrategy) GetName() string {
	return "RSI"
}

func (s *RSIStrategy) Evaluate(history *models.PriceHistory) TradeSignal {
	if history.Size() < s.Period+1 {
		return TradeSignal{Signal: SignalNone, Reason: "Insufficient data for RSI"}
	}

	prices := history.GetPrices(s.Period + 1)
	rsi := indicators.RSI(prices, s.Period)
	latest, _ := history.Latest()

	if rsi < s.OversoldLevel {
		return TradeSignal{
			Signal:   SignalBuy,
			Strength: SignalStrength(100 - rsi),
			Reason:   "RSI oversold",
			Price:    latest.Price,
		}
	}

	if rsi > s.OverboughtLevel {
		return TradeSignal{
			Signal:   SignalSell,
			Strength: SignalStrength(rsi),
			Reason:   "RSI overbought",
			Price:    latest.Price,
		}
	}

	return TradeSignal{Signal: SignalHold, Reason: "RSI neutral"}
}

// MovingAverageCrossStrategy - trend following with MA crossover
type MovingAverageCrossStrategy struct {
	FastPeriod int
	SlowPeriod int
}

func NewMovingAverageCrossStrategy(fastPeriod, slowPeriod int) *MovingAverageCrossStrategy {
	return &MovingAverageCrossStrategy{
		FastPeriod: fastPeriod,
		SlowPeriod: slowPeriod,
	}
}

func (s *MovingAverageCrossStrategy) GetName() string {
	return "MA Cross"
}

func (s *MovingAverageCrossStrategy) Evaluate(history *models.PriceHistory) TradeSignal {
	if history.Size() < s.SlowPeriod+2 {
		return TradeSignal{Signal: SignalNone, Reason: "Insufficient data for MA"}
	}

	prices := history.GetPrices(s.SlowPeriod + 2)
	latest, _ := history.Latest()

	// Current MAs
	fastMA := indicators.SMA(prices, s.FastPeriod)
	slowMA := indicators.SMA(prices, s.SlowPeriod)

	// Previous MAs (to detect crossover)
	prevFastMA := indicators.SMA(prices[:len(prices)-1], s.FastPeriod)
	prevSlowMA := indicators.SMA(prices[:len(prices)-1], s.SlowPeriod)

	// Bullish crossover: fast crosses above slow
	if prevFastMA <= prevSlowMA && fastMA > slowMA {
		strength := ((fastMA - slowMA) / slowMA) * 1000
		return TradeSignal{
			Signal:   SignalBuy,
			Strength: SignalStrength(strength),
			Reason:   "Bullish MA crossover",
			Price:    latest.Price,
		}
	}

	// Bearish crossover: fast crosses below slow
	if prevFastMA >= prevSlowMA && fastMA < slowMA {
		strength := ((slowMA - fastMA) / slowMA) * 1000
		return TradeSignal{
			Signal:   SignalSell,
			Strength: SignalStrength(strength),
			Reason:   "Bearish MA crossover",
			Price:    latest.Price,
		}
	}

	return TradeSignal{Signal: SignalHold, Reason: "No MA crossover"}
}

// MultiStrategyEngine combines multiple strategies
type MultiStrategyEngine struct {
	Strategies []Strategy
	Weights    map[string]float64
}

func NewMultiStrategyEngine(strategies []Strategy, weights map[string]float64) *MultiStrategyEngine {
	return &MultiStrategyEngine{
		Strategies: strategies,
		Weights:    weights,
	}
}
func (mse *MultiStrategyEngine) GetName() string {
	return "Multi-Strategy"
}
// Evaluate runs all strategies and returns consensus signal
func (mse *MultiStrategyEngine) Evaluate(history *models.PriceHistory) TradeSignal {
	buyScore := 0.0
	sellScore := 0.0
	reasons := []string{}

	for _, strategy := range mse.Strategies {
		signal := strategy.Evaluate(history)
		weight := mse.Weights[strategy.GetName()]
		if weight == 0 {
			weight = 1.0 // Default weight
		}

		switch signal.Signal {
		case SignalBuy:
			buyScore += float64(signal.Strength) * weight
			reasons = append(reasons, strategy.GetName()+": "+signal.Reason)
		case SignalSell:
			sellScore += float64(signal.Strength) * weight
			reasons = append(reasons, strategy.GetName()+": "+signal.Reason)
		}
	}

	latest, _ := history.Latest()

	// Require significant consensus to trade
	threshold := 50.0
	if buyScore > sellScore && buyScore > threshold {
		return TradeSignal{
			Signal:   SignalBuy,
			Strength: SignalStrength(buyScore),
			Reason:   "Multi-strategy consensus",
			Price:    latest.Price,
		}
	}

	if sellScore > buyScore && sellScore > threshold {
		return TradeSignal{
			Signal:   SignalSell,
			Strength: SignalStrength(sellScore),
			Reason:   "Multi-strategy consensus",
			Price:    latest.Price,
		}
	}

	return TradeSignal{
		Signal: SignalHold,
		Reason: "No consensus",
		Price:  latest.Price,
	}
}
