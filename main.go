package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gold-bot/datafeed"
	"gold-bot/engine"
	"gold-bot/models"
	"gold-bot/risk"
	"gold-bot/server"
	"gold-bot/strategy"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}

	// Get API key
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		log.Fatal("❌ Missing API_KEY environment variable")
	}

	// Configuration
	initialBalance := 10000.0
	tickRate := 30 * time.Second
	historySize := 500

	log.Println("🏦 Initializing Gold Trading Bot...")
	log.Printf("💰 Initial Balance: $%.2f", initialBalance)

	// Initialize components
	trader := models.NewTrader(initialBalance)
	riskManager := risk.NewRiskManager(initialBalance)
	
	// Create data feed
	goldAPI := datafeed.NewGoldAPIFeed(apiKey)
	priceFetcher := datafeed.NewPriceFetcher(goldAPI)
	priceStream := datafeed.NewPriceStream(priceFetcher, tickRate, historySize)

	// Initialize trading strategy
	// You can swap this out with different strategies
	tradingStrategy := createDefaultStrategy()

	// Start web server BEFORE creating engine (so we can reference it in callbacks)
	webServer := server.NewServer(trader, tickRate)
	
	// Create trading engine
	tradingEngine := engine.NewTradingEngine(
		trader,
		riskManager,
		tradingStrategy,
		priceStream,
	)

	tradingEngine.OnPriceUpdate = func(price *models.PricePoint) {
		webServer.BroadcastUpdate(trader.GetState())
	}

	tradingEngine.OnTradeOpen = func(trade *models.Trade) {
		log.Printf("📊 Trade opened: %+v", trade)
	}

	tradingEngine.OnTradeClose = func(trade *models.Trade) {
		log.Printf("📊 Trade closed: %+v", trade)
		log.Printf("📈 Account Balance: $%.2f", trader.AccountBalance)
	}

	// Start trading engine
	tradingEngine.Start()

	// Start web server in goroutine
	go func() {
		if err := webServer.Start(":8080"); err != nil {
			log.Fatalf("❌ Server error: %v", err)
		}
	}()

	log.Println("✅ Trading bot started successfully")
	log.Println("📡 WebSocket server running on :8080")
	log.Println("🌐 Health check: http://localhost:8080/health")
	log.Println("📊 Status endpoint: http://localhost:8080/status")

	// Wait for shutdown signal
	waitForShutdown(tradingEngine)
}

// createDefaultStrategy creates the default trading strategy
func createDefaultStrategy() strategy.Strategy {
	strategies := []strategy.Strategy{
		strategy.NewMomentumStrategy(0.5),
		strategy.NewRSIStrategy(14, 30, 70),
		strategy.NewMovingAverageCrossStrategy(10, 30),
	}

	weights := map[string]float64{
		"Momentum": 1.0,
		"RSI":      1.5,
		"MA Cross": 1.0,
	}

	return strategy.NewMultiStrategyEngine(strategies, weights)
}

// waitForShutdown waits for SIGINT or SIGTERM and gracefully shuts down
func waitForShutdown(tradingEngine *engine.TradingEngine) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	log.Println("\n🛑 Shutdown signal received...")

	// Graceful shutdown
	tradingEngine.Stop()

	log.Println("👋 Trading bot stopped. Goodbye!")
	os.Exit(0)
}
