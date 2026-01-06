package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"youflac/backend"
	"youflac/internal/api"
)

func main() {
	log.Println("YouFlac Server starting...")

	// Load config (env vars override file config)
	config, err := backend.LoadConfigWithEnv()
	if err != nil {
		log.Printf("Warning: Could not load config: %v, using defaults", err)
		config = backend.GetDefaultConfig()
	}

	// Ensure output directory exists
	outputDir := config.OutputDirectory
	if outputDir == "" {
		outputDir = backend.GetDefaultOutputDirectory()
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Printf("Warning: Could not create output directory: %v", err)
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize queue
	queue := backend.NewQueue(ctx, config.ConcurrentDownloads)

	// Initialize history
	history := backend.NewHistory()

	// Initialize file index
	dataPath := backend.GetDataPathWithEnv()
	fileIndex := backend.NewFileIndex(dataPath)
	go func() {
		if err := fileIndex.ScanDirectory(outputDir); err != nil {
			log.Printf("Warning: Could not scan output directory: %v", err)
		}
	}()

	// Create and configure server
	server := api.NewServer(config, queue, history, fileIndex)

	// Set queue progress callback to broadcast via WebSocket
	queue.SetProgressCallback(func(event backend.QueueEvent) {
		server.BroadcastQueueEvent(event)
	})

	// Start queue processing
	queue.StartProcessing()

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down...")
		cancel()
		queue.StopProcessing()
		queue.SaveQueue()
		server.Shutdown()
	}()

	// Get port from env or default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server listening on :%s", port)
	if err := server.Listen(":" + port); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
