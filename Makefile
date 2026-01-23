# Claude Build Analyzer - Tri-Server MCP Build System

# Variables
GO_VERSION := 1.21
BUILD_DIR := .
BINARIES := build-analyzer ocp-metadata-server jenkins-server

# Default target
.PHONY: all
all: build

# Build all three servers
.PHONY: build
build: build-analyzer ocp-metadata-server jenkins-server

# Build the BigQuery-focused build analyzer server
.PHONY: build-analyzer
build-analyzer:
	@echo "Building Build Analyzer server..."
	go build -o $(BUILD_DIR)/build-analyzer ./cmd/build-analyzer

# Build the metadata-focused OCP server
.PHONY: ocp-metadata-server
ocp-metadata-server:
	@echo "Building OCP Metadata server..."
	go build -o $(BUILD_DIR)/ocp-metadata-server ./cmd/ocp-metadata-server

# Build the Jenkins-focused server for Konflux build logs
.PHONY: jenkins-server
jenkins-server:
	@echo "Building Jenkins server..."
	go build -o $(BUILD_DIR)/jenkins-server ./cmd/jenkins-server

# Clean built binaries
.PHONY: clean
clean:
	@echo "Cleaning binaries..."
	rm -f $(BUILD_DIR)/build-analyzer $(BUILD_DIR)/ocp-metadata-server $(BUILD_DIR)/jenkins-server

# Test both servers
.PHONY: test
test:
	@echo "Running tests..."
	go test ./...

# Test Build Analyzer server independently
.PHONY: test-build-analyzer
test-build-analyzer: build-analyzer
	@echo "Testing Build Analyzer server..."
	@echo '{"jsonrpc": "2.0", "id": 1, "method": "tools/list"}' | \
		GOOGLE_APPLICATION_CREDENTIALS=./.credentials/google.json ./build-analyzer

# Test OCP Metadata server independently
.PHONY: test-ocp-metadata
test-ocp-metadata: ocp-metadata-server
	@echo "Testing OCP Metadata server..."
	@echo '{"jsonrpc": "2.0", "id": 1, "method": "tools/list"}' | ./ocp-metadata-server

# Test Jenkins server independently
.PHONY: test-jenkins
test-jenkins: jenkins-server
	@echo "Testing Jenkins server..."
	@echo '{"jsonrpc": "2.0", "id": 1, "method": "tools/list"}' | ./jenkins-server

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Lint code
.PHONY: lint
lint:
	@echo "Linting code..."
	golint ./...

# Vet code
.PHONY: vet
vet:
	@echo "Vetting code..."
	go vet ./...

# Run all checks
.PHONY: check
check: fmt vet test

# Install dependencies
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Show build information
.PHONY: info
info:
	@echo "Claude Build Analyzer - Tri-Server MCP System"
	@echo "Go version: $(shell go version)"
	@echo "Build targets:"
	@echo "  - build-analyzer: BigQuery-focused server (cmd/build-analyzer)"
	@echo "  - ocp-metadata-server: Git metadata-focused server (cmd/ocp-metadata-server)"
	@echo "  - jenkins-server: Jenkins API-focused server (cmd/jenkins-server)"
	@echo ""
	@echo "Available commands:"
	@echo "  make build           - Build all three servers"
	@echo "  make build-analyzer  - Build BigQuery server only"
	@echo "  make ocp-metadata-server - Build metadata server only"
	@echo "  make jenkins-server  - Build Jenkins server only"
	@echo "  make test           - Run all tests"
	@echo "  make test-build-analyzer - Test BigQuery server"
	@echo "  make test-ocp-metadata - Test metadata server"
	@echo "  make test-jenkins    - Test Jenkins server"
	@echo "  make clean          - Remove binaries"
	@echo "  make check          - Format, vet, and test"

# Architecture verification
.PHONY: verify-structure
verify-structure:
	@echo "Verifying tri-server architecture..."
	@test -f cmd/build-analyzer/main.go || (echo "ERROR: Missing cmd/build-analyzer/main.go"; exit 1)
	@test -f cmd/ocp-metadata-server/main.go || (echo "ERROR: Missing cmd/ocp-metadata-server/main.go"; exit 1)
	@test -f cmd/jenkins-server/main.go || (echo "ERROR: Missing cmd/jenkins-server/main.go"; exit 1)
	@test -f internal/build-analyzer-server/server.go || (echo "ERROR: Missing internal/build-analyzer-server/server.go"; exit 1)
	@test -f internal/metadata-server/server.go || (echo "ERROR: Missing internal/metadata-server/server.go"; exit 1)
	@test -f internal/jenkins-server/server.go || (echo "ERROR: Missing internal/jenkins-server/server.go"; exit 1)
	@test -f internal/jenkins/client.go || (echo "ERROR: Missing internal/jenkins/client.go"; exit 1)
	@test -f pkg/shared/mcp.go || (echo "ERROR: Missing pkg/shared/mcp.go"; exit 1)
	@test -f pkg/shared/bigquery.go || (echo "ERROR: Missing pkg/shared/bigquery.go"; exit 1)
	@test ! -d internal/mcp || (echo "ERROR: Old monolithic server directory still exists"; exit 1)
	@test ! -d internal/metadata || (echo "ERROR: Old metadata directory still exists"; exit 1)
	@test ! -d pkg/types || (echo "ERROR: Old types directory still exists"; exit 1)
	@echo "✓ Tri-server architecture verification passed"

# Help
.PHONY: help
help: info