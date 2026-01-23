package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	jenkinsserver "github.com/lgarciaaco/claude-build-analyzer/internal/jenkins-server"
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

	// Create and initialize Jenkins MCP server
	server, err := jenkinsserver.NewServer(ctx)
	if err != nil {
		log.Fatalf("Failed to create Jenkins MCP server: %v", err)
	}
	defer server.Close()

	// Initialize server
	if err := server.Initialize(); err != nil {
		log.Fatalf("Failed to initialize Jenkins MCP server: %v", err)
	}

	// Run the MCP server
	log.Println("Claude Jenkins MCP Server starting...")
	if err := server.Run(); err != nil {
		log.Fatalf("Jenkins MCP server failed: %v", err)
	}

	log.Println("Claude Jenkins MCP Server stopped")
}