package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	metadataserver "github.com/lgarciaaco/claude-build-analyzer/internal/metadata-server"
)

func main() {
	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("Received shutdown signal...")
		cancel()
	}()

	// Create and initialize server
	server, err := metadataserver.NewServer(ctx)
	if err != nil {
		log.Fatalf("Failed to create metadata server: %v", err)
	}
	defer server.Close()

	// Initialize repositories
	if err := server.Initialize(); err != nil {
		log.Fatalf("Failed to initialize metadata server: %v", err)
	}

	log.Println("OCP Metadata Server starting...")

	// Run server
	if err := server.Run(); err != nil {
		log.Fatalf("Server error: %v", err)
	}

	log.Println("OCP Metadata Server stopped")
}
