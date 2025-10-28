package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"bytes"

	"github.com/gorilla/websocket"
)


func mockAPI(price float64) *httptest.Server  {
	handler := http.HandlerFunc(func (w http.ResponseWriter, r *http.Request)  {
		json.NewEncoder(w).Encode(GoldPriceResponse{
			Price: price,
		})
	})
	return httptest.NewServer(handler)
}


func TestFetchPrice(t *testing.T)  {
	ts := mockAPI(1234.56)
	defer ts.Close()

	price := fetchPrice("FAKEKEY", ts.URL)

	if price != 1234.56{
		t.Fatalf("Expected price 1234.56, got %.2f", price)
	}
}

func TestEvaluateStrategy(t *testing.T)  {
	tr := Trader{LastPrice: 1000, Position: "NONE"}

	evaluateStrategy(&tr, 1010)
	if tr.Position != "LONG"{
		t.Errorf("Expected LONG position, got %s", tr.Position)
	}

	evaluateStrategy(&tr, 990)
	if tr.Position != "NONE"{
		t.Errorf("Expected NONE position, got %s", tr.Position)
	}
}

func TestUpdatePrice(t *testing.T)  {
	tr := Trader{}
	tr.UpdatePrice(2000)

	if tr.LastPrice != 2000{
		t.Errorf("Expected 2000, got %.2f", tr.LastPrice)
	}
	if tr.Timestamp == 0 {
		t.Error("Expected timestamp to be set")
	}
}

func TestWebSocketConnection(t *testing.T)  {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleWS(w,r)
	}))
	defer server.Close()

	url := "ws" + server.URL[4:]

	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil{
		t.Fatalf("Websocket connection failed: %v", err)
	}
	ws.Close()
}

func TestTickRateupdate(t *testing.T)  {
	 // replace the global intervalChan with a buffered channel for the test
    oldChan := intervalChan
    intervalChan = make(chan time.Duration, 1) // buffered
    defer func(){ intervalChan = oldChan }()

    req := httptest.NewRequest("POST", "/tickrate",
        jsonBody(t, map[string]int{"tick_rate_sec": 45}))
    w := httptest.NewRecorder()

    handleTickRate(w, req)

    if w.Code != http.StatusOK {
        t.Fatalf("Expected 200, got %d", w.Code)
    }

    config.RLock()
    if config.TickRate != 45*time.Second {
        t.Errorf("Tickrate not updated: got %v", config.TickRate)
    }
    config.RUnlock()
}

func jsonBody(t *testing.T, data interface{}) *bytes.Buffer  {
	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(data)
	return b
}