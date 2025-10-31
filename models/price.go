package models

import (
	"sync"
	"time"

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

type PricePoint struct{
	Price float64
	Timestamp time.Time
	High float64
	Low float64
	Volume float64
}

type PriceHistory struct{
	Points []PricePoint
	MaxSize int
	mu sync.RWMutex
}

func NewPriceHistory(maxSize int) *PriceHistory{
	return &PriceHistory{
		Points: make([]PricePoint, 0, maxSize),
		MaxSize: maxSize,
	}
}

func  (ph *PriceHistory) Add(point PricePoint)  {
	ph.mu.Lock()
	defer ph.mu.Unlock()

	ph.Points = append(ph.Points, point)

	if len(ph.Points) > ph.MaxSize{
		ph.Points = ph.Points[1:]
	}
}

func (ph *PriceHistory) GetLast(n int)  []PricePoint {
	ph.mu.RLock()
	defer ph.mu.RUnlock()

	if n > len(ph.Points) {
		n =  len(ph.Points)
	}

	result := make([]PricePoint, n)
	copy(result, ph.Points[len(ph.Points)-n:])
	return result
}

func (ph *PriceHistory)GetPrices(n int) []float64 {
	points := ph.GetLast(n)
	prices:= make([]float64, len(points))
	for i, p := range points {
		prices[i] = p.Price
	}
	return prices
}

func (ph *PriceHistory) Latest() (PricePoint, bool) {
	ph.mu.Lock()
	defer ph.mu.RUnlock()

	if len(ph.Points) == 0 {
		return PricePoint{}, false
	}
	return ph.Points[len(ph.Points)-1], true
}

