package server

import (
	"encoding/json"
	"gold-bot/models"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Configure this properly for production
	},
}

// Server handles WebSocket connections and HTTP endpoints
type Server struct {
	clients   map[*websocket.Conn]bool
	clientsMu sync.Mutex
	trader    *models.Trader
	config    *Config
}

// Config holds server configuration
type Config struct {
	TickRate time.Duration
	mu       sync.RWMutex
}

// NewServer creates a new WebSocket server
func NewServer(trader *models.Trader, tickRate time.Duration) *Server {
	return &Server{
		clients: make(map[*websocket.Conn]bool),
		trader:  trader,
		config: &Config{
			TickRate: tickRate,
		},
	}
}

// HandleWebSocket handles WebSocket connections
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	s.clientsMu.Lock()
	s.clients[conn] = true
	s.clientsMu.Unlock()

	log.Printf("New WebSocket client connected. Total clients: %d", len(s.clients))

	// Send initial state
	s.sendToClient(conn, s.trader.GetState())

	// Read messages (keeps connection alive)
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			s.clientsMu.Lock()
			delete(s.clients, conn)
			s.clientsMu.Unlock()
			log.Printf("Client disconnected. Total clients: %d", len(s.clients))
			break
		}
	}
}

// HandleTickRate handles GET/POST requests for tick rate configuration
func (s *Server) HandleTickRate(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.config.mu.RLock()
		tickRate := s.config.TickRate
		s.config.mu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"tick_rate_seconds": tickRate.Seconds(),
			"tick_rate_ms":      tickRate.Milliseconds(),
		})

	case http.MethodPost:
		var req struct {
			TickRateSeconds int `json:"tick_rate_seconds"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.TickRateSeconds < 1 {
			http.Error(w, "Tick rate must be at least 1 second", http.StatusBadRequest)
			return
		}

		newInterval := time.Duration(req.TickRateSeconds) * time.Second
		s.config.mu.Lock()
		s.config.TickRate = newInterval
		s.config.mu.Unlock()

		log.Printf("Tick rate updated to: %v", newInterval)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "success",
			"message": "Tick rate updated",
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleStatus returns current trading status
func (s *Server) HandleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.trader.GetState())
}

// BroadcastUpdate broadcasts update to all connected clients
func (s *Server) BroadcastUpdate(data interface{}) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	for conn := range s.clients {
		if err := conn.WriteJSON(data); err != nil {
			log.Printf("Error broadcasting to client: %v", err)
			conn.Close()
			delete(s.clients, conn)
		}
	}
}

// sendToClient sends data to a specific client
func (s *Server) sendToClient(conn *websocket.Conn, data interface{}) {
	if err := conn.WriteJSON(data); err != nil {
		log.Printf("Error sending to client: %v", err)
	}
}

// GetTickRate returns current tick rate (thread-safe)
func (s *Server) GetTickRate() time.Duration {
	s.config.mu.RLock()
	defer s.config.mu.RUnlock()
	return s.config.TickRate
}

// GetClientCount returns number of connected clients
func (s *Server) GetClientCount() int {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	return len(s.clients)
}

// SetupRoutes configures all HTTP routes
func (s *Server) SetupRoutes() *http.ServeMux {
	mux := http.NewServeMux()
	
	mux.HandleFunc("/ws", s.HandleWebSocket)
	mux.HandleFunc("/tickrate", s.HandleTickRate)
	mux.HandleFunc("/status", s.HandleStatus)
	
	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":      "healthy",
			"clients":     s.GetClientCount(),
			"tick_rate":   s.GetTickRate().String(),
			"server_time": time.Now().Unix(),
		})
	})

	return mux
}

// Start starts the HTTP server
func (s *Server) Start(addr string) error {
	mux := s.SetupRoutes()
	log.Printf("Server starting on %s", addr)
	return http.ListenAndServe(addr, mux)
}
