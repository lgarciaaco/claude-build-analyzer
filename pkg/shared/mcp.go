package shared

import (
	"context"
	"encoding/json"
	"fmt"
)

// MCPServer defines the interface that all MCP servers must implement
type MCPServer interface {
	Initialize() error
	Run() error
	Close() error
	GetToolList() ToolList
	ExecuteTool(ctx context.Context, call ToolCall) (ToolResult, error)
}

// MCPRequest represents an incoming MCP request
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// MCPResponse represents an MCP response
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP error
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ToolList represents the list of available tools
type ToolList struct {
	Tools []Tool `json:"tools"`
}

// Tool represents a single MCP tool
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

// InputSchema represents the input schema for a tool
type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

// Property represents a schema property
type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Items       *Items `json:"items,omitempty"`
}

// Items represents array items schema
type Items struct {
	Type string `json:"type"`
}

// ToolCall represents a tool call request
type ToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ToolResult represents a tool call result
type ToolResult struct {
	Content []Content `json:"content"`
}

// Content represents result content
type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// InitializeResponse represents the MCP initialize response
type InitializeResponse struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	ServerInfo      ServerInfo             `json:"serverInfo"`
	Capabilities    map[string]interface{} `json:"capabilities"`
}

// ServerInfo represents server information
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ParseJSONResult parses a ToolResult into a struct
func ParseJSONResult(result ToolResult, target interface{}) error {
	if len(result.Content) == 0 {
		return fmt.Errorf("no content in result")
	}
	return json.Unmarshal([]byte(result.Content[0].Text), target)
}
