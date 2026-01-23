package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	buildanalyzer "github.com/lgarciaaco/claude-build-analyzer/internal/build-analyzer-server"
)

func main() {
	// Set up logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(os.Stderr) // Log to stderr so stdout is clean for MCP communication

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-signalCh
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	// Create and initialize Build Analyzer MCP server
	server, err := buildanalyzer.NewServer(ctx)
	if err != nil {
		log.Fatalf("Failed to create Build Analyzer MCP server: %v", err)
	}
	defer server.Close()

	// Initialize server (BigQuery client, etc.)
	if err := server.Initialize(); err != nil {
		log.Fatalf("Failed to initialize Build Analyzer MCP server: %v", err)
	}

	// Run the MCP server
	log.Println("Claude Build Analyzer MCP Server starting...")
	if err := server.Run(); err != nil {
		log.Fatalf("Build Analyzer MCP server failed: %v", err)
	}

	log.Println("Claude Build Analyzer MCP Server stopped")
}
