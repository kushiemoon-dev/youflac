package api

import (
	"log"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/websocket/v2"

	"youflac/backend"
)

// Server represents the HTTP API server
type Server struct {
	app       *fiber.App
	config    *backend.Config
	queue     *backend.Queue
	history   *backend.History
	fileIndex *backend.FileIndex
	wsHub     *WebSocketHub
}

// NewServer creates a new API server instance
func NewServer(config *backend.Config, queue *backend.Queue, history *backend.History, fileIndex *backend.FileIndex) *Server {
	app := fiber.New(fiber.Config{
		AppName:      "YouFlac Server",
		ServerHeader: "YouFlac",
		BodyLimit:    50 * 1024 * 1024, // 50MB
	})

	// Create WebSocket hub
	wsHub := NewWebSocketHub()
	go wsHub.Run()

	server := &Server{
		app:       app,
		config:    config,
		queue:     queue,
		history:   history,
		fileIndex: fileIndex,
		wsHub:     wsHub,
	}

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${method} ${path} (${latency})\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))

	// Setup routes
	server.setupRoutes()

	return server
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Health check
	s.app.Get("/api/health", s.handleHealth)

	// API routes
	api := s.app.Group("/api")

	// Queue routes
	api.Get("/queue", s.handleGetQueue)
	api.Post("/queue", s.handleAddToQueue)
	api.Get("/queue/stats", s.handleGetQueueStats)
	api.Post("/queue/clear", s.handleClearCompleted)
	api.Post("/queue/retry", s.handleRetryFailed)
	api.Get("/queue/:id", s.handleGetQueueItem)
	api.Delete("/queue/:id", s.handleRemoveFromQueue)
	api.Post("/queue/:id/cancel", s.handleCancelQueueItem)
	api.Put("/queue/:id/move", s.handleMoveQueueItem)

	// Playlist routes
	api.Post("/playlist", s.handleAddPlaylistToQueue)

	// Config routes
	api.Get("/config", s.handleGetConfig)
	api.Post("/config", s.handleSaveConfig)
	api.Get("/config/default-output", s.handleGetDefaultOutput)

	// History routes
	api.Get("/history", s.handleGetHistory)
	api.Get("/history/stats", s.handleGetHistoryStats)
	api.Get("/history/search", s.handleSearchHistory)
	api.Delete("/history/:id", s.handleDeleteHistoryEntry)
	api.Post("/history/clear", s.handleClearHistory)
	api.Post("/history/:id/redownload", s.handleRedownloadFromHistory)

	// Video/URL routes
	api.Post("/video/parse", s.handleParseURL)
	api.Get("/video/info", s.handleGetVideoInfo)
	api.Post("/video/match", s.handleFindAudioMatch)

	// Files routes
	api.Get("/files", s.handleListFiles)
	api.Get("/files/playlists", s.handleGetPlaylistFolders)
	api.Post("/files/reorganize", s.handleReorganizePlaylist)
	api.Post("/files/flatten", s.handleFlattenPlaylist)

	// Analyzer routes
	api.Post("/analyze", s.handleAnalyzeAudio)
	api.Post("/analyze/spectrogram", s.handleGenerateSpectrogram)
	api.Post("/analyze/waveform", s.handleGenerateWaveform)

	// Lyrics routes
	api.Get("/lyrics", s.handleFetchLyrics)
	api.Post("/lyrics/embed", s.handleEmbedLyrics)
	api.Post("/lyrics/save", s.handleSaveLRCFile)

	// Static image serving (for spectrograms, thumbnails)
	api.Get("/image", s.handleGetImage)

	// Version
	api.Get("/version", s.handleGetVersion)

	// WebSocket endpoint
	s.app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	s.app.Get("/ws", websocket.New(s.handleWebSocket))

	// Static files (React build) - must be last
	s.app.Static("/", "./frontend/dist")

	// SPA fallback - serve index.html for all non-API routes
	s.app.Get("/*", func(c *fiber.Ctx) error {
		return c.SendFile("./frontend/dist/index.html")
	})
}

// Listen starts the HTTP server
func (s *Server) Listen(addr string) error {
	return s.app.Listen(addr)
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	s.wsHub.Close()
	return s.app.Shutdown()
}

// BroadcastQueueEvent sends a queue event to all connected WebSocket clients
func (s *Server) BroadcastQueueEvent(event backend.QueueEvent) {
	s.wsHub.Broadcast(event)
}

// WebSocketHub manages WebSocket connections
type WebSocketHub struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan interface{}
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mu         sync.RWMutex
	done       chan struct{}
}

// NewWebSocketHub creates a new WebSocket hub
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan interface{}, 256),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		done:       make(chan struct{}),
	}
}

// Run starts the WebSocket hub
func (h *WebSocketHub) Run() {
	for {
		select {
		case <-h.done:
			return
		case conn := <-h.register:
			h.mu.Lock()
			h.clients[conn] = true
			h.mu.Unlock()
			log.Printf("WebSocket client connected (total: %d)", len(h.clients))
		case conn := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[conn]; ok {
				delete(h.clients, conn)
				conn.Close()
			}
			h.mu.Unlock()
			log.Printf("WebSocket client disconnected (total: %d)", len(h.clients))
		case message := <-h.broadcast:
			h.mu.RLock()
			for conn := range h.clients {
				if err := conn.WriteJSON(message); err != nil {
					log.Printf("WebSocket write error: %v", err)
					h.mu.RUnlock()
					h.unregister <- conn
					h.mu.RLock()
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends a message to all connected clients
func (h *WebSocketHub) Broadcast(message interface{}) {
	select {
	case h.broadcast <- message:
	default:
		log.Println("WebSocket broadcast channel full, dropping message")
	}
}

// Close shuts down the hub
func (h *WebSocketHub) Close() {
	close(h.done)
	h.mu.Lock()
	for conn := range h.clients {
		conn.Close()
	}
	h.mu.Unlock()
}

// handleWebSocket handles WebSocket connections
func (s *Server) handleWebSocket(c *websocket.Conn) {
	s.wsHub.register <- c
	defer func() {
		s.wsHub.unregister <- c
	}()

	for {
		_, _, err := c.ReadMessage()
		if err != nil {
			break
		}
		// We don't process incoming messages, just keep connection alive
	}
}
