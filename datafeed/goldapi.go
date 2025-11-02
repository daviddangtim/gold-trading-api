package datafeed

import (
	"encoding/json"
	"errors"
	"fmt"
	"gold-bot/models"
	"log"
	"net/http"
	"time"
)

// DataFeed interface for different price data sources
type DataFeed interface {
	FetchPrice() (*models.PricePoint, error)
	GetName() string
}

// GoldAPIFeed fetches data from GoldAPI.io
type GoldAPIFeed struct {
	APIKey   string
	BaseURL  string
	Symbol   string
	Currency string
	Client   *http.Client
}

// NewGoldAPIFeed creates a new GoldAPI data feed
func NewGoldAPIFeed(apiKey string) *GoldAPIFeed {
	return &GoldAPIFeed{
		APIKey:   apiKey,
		BaseURL:  "https://www.goldapi.io/api",
		Symbol:   "XAU",
		Currency: "USD",
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetName returns the name of this data feed
func (g *GoldAPIFeed) GetName() string {
	return "GoldAPI"
}

// FetchPrice fetches current gold price from GoldAPI
func (g *GoldAPIFeed) FetchPrice() (*models.PricePoint, error) {
	url := fmt.Sprintf("%s/%s/%s", g.BaseURL, g.Symbol, g.Currency)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-access-token", g.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch price: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}

	var apiResponse models.GoldPriceResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to PricePoint
	pricePoint := &models.PricePoint{
		Price:     apiResponse.Price,
		Timestamp: time.Unix(apiResponse.Timestamp, 0),
		High:      apiResponse.HighPrice,
		Low:       apiResponse.LowPrice,
	}

	return pricePoint, nil
}

// PriceFetcher manages data fetching with retry logic and error handling
type PriceFetcher struct {
	DataFeed     DataFeed
	RetryAttempts int
	RetryDelay    time.Duration
}

// NewPriceFetcher creates a new price fetcher with retry logic
func NewPriceFetcher(dataFeed DataFeed) *PriceFetcher {
	return &PriceFetcher{
		DataFeed:     dataFeed,
		RetryAttempts: 3,
		RetryDelay:    2 * time.Second,
	}
}

// Fetch fetches price with retry logic
func (pf *PriceFetcher) Fetch() (*models.PricePoint, error) {
	var lastErr error
	
	for attempt := 0; attempt < pf.RetryAttempts; attempt++ {
		if attempt > 0 {
			log.Printf("Retry attempt %d/%d after error: %v", attempt+1, pf.RetryAttempts, lastErr)
			time.Sleep(pf.RetryDelay)
		}

		pricePoint, err := pf.DataFeed.FetchPrice()
		if err == nil {
			return pricePoint, nil
		}

		lastErr = err
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", pf.RetryAttempts, lastErr)
}

// PriceStream manages continuous price streaming
type PriceStream struct {
	Fetcher      *PriceFetcher
	Interval     time.Duration
	PriceHistory *models.PriceHistory
	ticker       *time.Ticker
	stopChan     chan struct{}
	priceChan    chan *models.PricePoint
	errorChan    chan error
}

// NewPriceStream creates a new price stream
func NewPriceStream(fetcher *PriceFetcher, interval time.Duration, historySize int) *PriceStream {
	return &PriceStream{
		Fetcher:      fetcher,
		Interval:     interval,
		PriceHistory: models.NewPriceHistory(historySize),
		stopChan:     make(chan struct{}),
		priceChan:    make(chan *models.PricePoint, 10),
		errorChan:    make(chan error, 10),
	}
}

// Start begins streaming prices
func (ps *PriceStream) Start() {
	ps.ticker = time.NewTicker(ps.Interval)
	
	go func() {
		// Fetch immediately on start
		ps.fetchAndNotify()

		for {
			select {
			case <-ps.ticker.C:
				ps.fetchAndNotify()
			case <-ps.stopChan:
				ps.ticker.Stop()
				return
			}
		}
	}()
}

// fetchAndNotify fetches price and notifies subscribers
func (ps *PriceStream) fetchAndNotify() {
	pricePoint, err := ps.Fetcher.Fetch()
	if err != nil {
		log.Printf("Error fetching price: %v", err)
		select {
		case ps.errorChan <- err:
		default:
		}
		return
	}

	// Add to history
	ps.PriceHistory.Add(*pricePoint)

	// Notify subscribers
	select {
	case ps.priceChan <- pricePoint:
	default:
		log.Println("Warning: Price channel full, dropping price update")
	}
}

// Stop stops the price stream
func (ps *PriceStream) Stop() {
	close(ps.stopChan)
}

// PriceChan returns the channel for receiving price updates
func (ps *PriceStream) PriceChan() <-chan *models.PricePoint {
	return ps.priceChan
}

// ErrorChan returns the channel for receiving errors
func (ps *PriceStream) ErrorChan() <-chan error {
	return ps.errorChan
}

// GetHistory returns the price history
func (ps *PriceStream) GetHistory() *models.PriceHistory {
	return ps.PriceHistory
}

// UpdateInterval changes the streaming interval
func (ps *PriceStream) UpdateInterval(newInterval time.Duration) error {
	if newInterval < 1*time.Second {
		return errors.New("interval must be at least 1 second")
	}

	ps.Interval = newInterval
	if ps.ticker != nil {
		ps.ticker.Reset(newInterval)
	}

	return nil
}

// GetLatestPrice returns the most recent price point
func (ps *PriceStream) GetLatestPrice() (*models.PricePoint, error) {
	point, ok := ps.PriceHistory.Latest()
	if !ok {
		return nil, errors.New("no price data available")
	}
	return &point, nil
}
