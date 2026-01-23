package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	jiraserver "github.com/lgarciaaco/claude-build-analyzer/internal/jira-server"
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

	// Create and initialize JIRA MCP server
	server, err := jiraserver.NewServer(ctx)
	if err != nil {
		log.Fatalf("Failed to create JIRA MCP server: %v", err)
	}
	defer server.Close()

	// Initialize server
	if err := server.Initialize(); err != nil {
		log.Fatalf("Failed to initialize JIRA MCP server: %v", err)
	}

	// Run the MCP server
	log.Println("Claude JIRA MCP Server starting...")
	if err := server.Run(); err != nil {
		log.Fatalf("JIRA MCP server failed: %v", err)
	}

	log.Println("Claude JIRA MCP Server stopped")
}
