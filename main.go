package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

type GoldPriceResponse struct {
	Timestamp      int64   `json:"timestamp"`
	Metal          string  `json:"metal"`
	Currency       string  `json:"currency"`
	Exchange       string  `json:"exchange"`
	Symbol         string  `json:"symbol"`
	PrevClosePrice float64 `json:"prev_close_price"`
	OpenPrice      float64 `json:"open_price"`
	LowPrice       float64 `json:"low_price"`
	HighPrice      float64 `json:"high_price"`
	OpenTime       int64   `json:"open_time"`
	Price          float64 `json:"price"`
	Change         float64 `json:"ch"`
	ChangePercent  float64 `json:"chp"`
	Ask            float64 `json:"ask"`
	Bid            float64 `json:"bid"`
	PriceGram24k   float64 `json:"price_gram_24k"`
	PriceGram22k   float64 `json:"price_gram_22k"`
	PriceGram21k   float64 `json:"price_gram_21k"`
	PriceGram20k   float64 `json:"price_gram_20k"`
	PriceGram18k   float64 `json:"price_gram_18k"`
	PriceGram16k   float64 `json:"price_gram_16k"`
	PriceGram14k   float64 `json:"price_gram_14k"`
	PriceGram10k   float64 `json:"price_gram_10k"`
}

type Trader struct {
	LastPrice float64 `json:"last_price"`
	Position  string  `json:"position"`
	Timestamp int64   `json:"timestamp"`
}
type Config struct {
	TickRate time.Duration `json:"tick_rate"`
	mu       sync.RWMutex    `json:"-"`
}

func (c *Config) Lock()    { c.mu.Lock() }
func (c *Config) Unlock()  { c.mu.Unlock() }
func (c *Config) RLock()   { c.mu.RLock() }
func (c *Config) RUnlock() { c.mu.RUnlock() }

var (
	updateInterval = time.Minute * 1
	intervalChan   = make(chan time.Duration)

	trader = Trader{LastPrice: 0, Position: "NONE"}
	config = Config{TickRate: 30 * time.Second}

	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	url = "https://www.goldapi.io/api/XAU/USD"

	clients   = make(map[*websocket.Conn]bool)
	clientsMu sync.Mutex
)

func (t *Trader) UpdatePrice(price float64) {
	t.LastPrice = price
	t.Timestamp = time.Now().Unix()
}

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found or not loaded")
	}
}

func main() {
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		log.Fatalf("Missing Gold API Key")
	}

	go startTrader(apiKey, &trader)

	http.HandleFunc("/ws", handleWS)

	http.HandleFunc("/tickrate", handleTickRate)

	log.Println("Server has started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Websocket upgrade failed:", err)
		return
	}
	defer conn.Close()

	clientsMu.Lock()
	clients[conn] = true
	clientsMu.Unlock()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			clientsMu.Lock()
			delete(clients, conn)
			clientsMu.Unlock()
			break
		}
	}
}

func handleTickRate(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		config.RLock()
		tickRate := config.TickRate
		config.RUnlock()
		json.NewEncoder(w).Encode(map[string]int{"tick_rate_sec": int(tickRate.Seconds())})
	case http.MethodPost:
		var newConfig struct {
			TickRateSec int `json:"tick_rate_sec"`
		}
		if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		newInterval := time.Duration(newConfig.TickRateSec) * time.Second

		config.Lock()
		config.TickRate = newInterval
		config.Unlock()

		intervalChan <- newInterval

		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func startTrader(apiKey string, trader *Trader) {
	ticker := time.NewTicker(config.TickRate)

	for {
		select {
		case <-ticker.C:
			price := fetchPrice(apiKey, url)
			evaluateStrategy(trader, price)
			trader.UpdatePrice(price)
			broadcastPrice(trader)
		case newInterval := <-intervalChan:
			log.Println("Tick interval updated to:", newInterval)
			ticker.Reset(newInterval)
		}
	}
}

func fetchPrice(apiKey string, url string) float64 {

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("x-access-token", apiKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("API error: %v\n", err)
		return 0
	}

	defer res.Body.Close()

	var data GoldPriceResponse
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		log.Printf("Decode error; %v\n", err)
		return 0
	}

	return data.Price
}

func evaluateStrategy(trader *Trader, price float64) {
	if price == 0 {
		return
	}

	if trader.LastPrice == 0 {
		trader.LastPrice = price
		return
	}

	change := ((price - trader.LastPrice) / trader.LastPrice) * 100

	if change > 0.5 && trader.Position == "NONE" {
		trader.Position = "LONG"
		fmt.Printf("📈 BUY @ %.2f (%.2f%% up)\n", price, change)
	}

	if change < -0.5 && trader.Position == "LONG" {
		trader.Position = "NONE"
		fmt.Printf("📉 SELL @ %.2f (%.2f%% down)\n", price, change)
	}

	fmt.Printf("💰 Price: %.2f | Change: %.2f%% | Position: %s\n",
		price, change, trader.Position)

	trader.LastPrice = price
}

func broadcastPrice(trader *Trader) {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	for conn := range clients {
		err := conn.WriteJSON(trader)
		if err != nil {
			log.Println("Websocket write failed:", err)
			conn.Close()
			delete(clients, conn)
		}
	}
}
