package shared

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
)

// BaseMCPServer provides common MCP server functionality
type BaseMCPServer struct {
	name         string
	version      string
	ctx          context.Context
	toolHandlers map[string]ToolHandler
	getToolList  func() ToolList
}

// ToolHandler represents a function that handles a specific tool call
type ToolHandler func(ctx context.Context, args map[string]interface{}) (ToolResult, error)

// NewBaseMCPServer creates a new base MCP server
func NewBaseMCPServer(name, version string, ctx context.Context, getToolList func() ToolList) *BaseMCPServer {
	return &BaseMCPServer{
		name:         name,
		version:      version,
		ctx:          ctx,
		toolHandlers: make(map[string]ToolHandler),
		getToolList:  getToolList,
	}
}

// RegisterTool registers a tool handler
func (s *BaseMCPServer) RegisterTool(name string, handler ToolHandler) {
	s.toolHandlers[name] = handler
}

// Run starts the MCP server on stdio
func (s *BaseMCPServer) Run() error {
	log.Printf("Starting MCP server %s on stdio...", s.name)

	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	requestCh := make(chan MCPRequest)
	errCh := make(chan error)

	// Start goroutine to handle stdin reading
	go func() {
		defer close(requestCh)
		defer close(errCh)

		for {
			var request MCPRequest
			if err := decoder.Decode(&request); err != nil {
				if err == io.EOF {
					return
				}
				select {
				case errCh <- err:
				case <-s.ctx.Done():
					return
				}
				continue
			}

			select {
			case requestCh <- request:
			case <-s.ctx.Done():
				return
			}
		}
	}()

	// Main event loop
	for {
		select {
		case <-s.ctx.Done():
			log.Println("Context cancelled, shutting down gracefully...")
			return nil
		case err, ok := <-errCh:
			if !ok {
				return nil
			}
			log.Printf("Failed to decode request: %v", err)
		case request, ok := <-requestCh:
			if !ok {
				return nil
			}
			response := s.handleRequest(request)
			if err := encoder.Encode(response); err != nil {
				log.Printf("Failed to encode response: %v", err)
			}
		}
	}
}

// handleRequest handles an incoming MCP request
func (s *BaseMCPServer) handleRequest(request MCPRequest) MCPResponse {
	switch request.Method {
	case "initialize":
		return s.handleInitialize(request)
	case "initialized":
		return s.handleInitialized(request)
	case "tools/list":
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      request.ID,
			Result:  s.getToolList(),
		}
	case "tools/call":
		return s.handleToolCall(request)
	default:
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      request.ID,
			Error: &MCPError{
				Code:    -32601,
				Message: fmt.Sprintf("Method not found: %s", request.Method),
			},
		}
	}
}

// handleInitialize handles the MCP initialize request
func (s *BaseMCPServer) handleInitialize(request MCPRequest) MCPResponse {
	return MCPResponse{
		JSONRPC: "2.0",
		ID:      request.ID,
		Result: InitializeResponse{
			ProtocolVersion: "2024-11-05",
			ServerInfo: ServerInfo{
				Name:    s.name,
				Version: s.version,
			},
			Capabilities: map[string]interface{}{
				"tools": map[string]interface{}{},
			},
		},
	}
}

// handleInitialized handles the MCP initialized notification
func (s *BaseMCPServer) handleInitialized(request MCPRequest) MCPResponse {
	return MCPResponse{
		JSONRPC: "2.0",
		ID:      request.ID,
		Result:  map[string]interface{}{},
	}
}

// handleToolCall handles a tool call request
func (s *BaseMCPServer) handleToolCall(request MCPRequest) MCPResponse {
	var toolCall ToolCall
	if err := json.Unmarshal(request.Params, &toolCall); err != nil {
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      request.ID,
			Error: &MCPError{
				Code:    -32602,
				Message: fmt.Sprintf("Invalid params: %v", err),
			},
		}
	}

	result, err := s.ExecuteTool(s.ctx, toolCall)
	if err != nil {
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      request.ID,
			Error: &MCPError{
				Code:    -32603,
				Message: fmt.Sprintf("Tool execution failed: %v", err),
			},
		}
	}

	return MCPResponse{
		JSONRPC: "2.0",
		ID:      request.ID,
		Result:  result,
	}
}

// ExecuteTool executes a tool call
func (s *BaseMCPServer) ExecuteTool(ctx context.Context, call ToolCall) (ToolResult, error) {
	handler, exists := s.toolHandlers[call.Name]
	if !exists {
		return ToolResult{}, fmt.Errorf("unknown tool: %s", call.Name)
	}

	return handler(ctx, call.Arguments)
}

// FormatJSONResult formats a result as JSON for tool response
func FormatJSONResult(result interface{}) (ToolResult, error) {
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return ToolResult{}, err
	}

	return ToolResult{
		Content: []Content{
			{
				Type: "text",
				Text: string(resultJSON),
			},
		},
	}, nil
}
