package shared

import (
	"context"
	"encoding/json"
	"testing"
)

func TestBaseMCPServer_RegisterTool(t *testing.T) {
	ctx := context.Background()
	getToolList := func() ToolList {
		return ToolList{Tools: []Tool{}}
	}

	server := NewBaseMCPServer("test-server", "1.0.0", ctx, getToolList)

	// Test tool registration
	testHandler := func(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
		return ToolResult{}, nil
	}

	server.RegisterTool("test_tool", testHandler)

	if _, exists := server.toolHandlers["test_tool"]; !exists {
		t.Error("Tool was not registered properly")
	}
}

func TestBaseMCPServer_ExecuteTool(t *testing.T) {
	ctx := context.Background()
	getToolList := func() ToolList {
		return ToolList{Tools: []Tool{}}
	}

	server := NewBaseMCPServer("test-server", "1.0.0", ctx, getToolList)

	// Register a test tool
	testHandler := func(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
		return ToolResult{
			Content: []Content{
				{Type: "text", Text: "test result"},
			},
		}, nil
	}

	server.RegisterTool("test_tool", testHandler)

	// Test tool execution
	call := ToolCall{
		Name:      "test_tool",
		Arguments: map[string]interface{}{"arg1": "value1"},
	}

	result, err := server.ExecuteTool(ctx, call)
	if err != nil {
		t.Fatalf("Tool execution failed: %v", err)
	}

	if len(result.Content) != 1 {
		t.Errorf("Expected 1 content item, got %d", len(result.Content))
	}

	if result.Content[0].Text != "test result" {
		t.Errorf("Expected 'test result', got '%s'", result.Content[0].Text)
	}
}

func TestBaseMCPServer_ExecuteTool_UnknownTool(t *testing.T) {
	ctx := context.Background()
	getToolList := func() ToolList {
		return ToolList{Tools: []Tool{}}
	}

	server := NewBaseMCPServer("test-server", "1.0.0", ctx, getToolList)

	// Test unknown tool execution
	call := ToolCall{
		Name:      "unknown_tool",
		Arguments: map[string]interface{}{},
	}

	_, err := server.ExecuteTool(ctx, call)
	if err == nil {
		t.Error("Expected error for unknown tool")
	}

	expectedError := "unknown tool: unknown_tool"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestFormatJSONResult(t *testing.T) {
	data := map[string]interface{}{
		"component": "ironic",
		"status":    "success",
		"count":     42,
	}

	result, err := FormatJSONResult(data)
	if err != nil {
		t.Fatalf("FormatJSONResult failed: %v", err)
	}

	if len(result.Content) != 1 {
		t.Errorf("Expected 1 content item, got %d", len(result.Content))
	}

	if result.Content[0].Type != "text" {
		t.Errorf("Expected content type 'text', got '%s'", result.Content[0].Type)
	}

	// Verify JSON can be parsed back
	var parsed map[string]interface{}
	err = json.Unmarshal([]byte(result.Content[0].Text), &parsed)
	if err != nil {
		t.Fatalf("Generated JSON is invalid: %v", err)
	}

	if parsed["component"] != "ironic" {
		t.Errorf("Expected component 'ironic', got '%v'", parsed["component"])
	}
}

func TestBaseMCPServer_handleInitialize(t *testing.T) {
	ctx := context.Background()
	getToolList := func() ToolList {
		return ToolList{Tools: []Tool{}}
	}

	server := NewBaseMCPServer("test-server", "1.0.0", ctx, getToolList)

	request := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{}`),
	}

	response := server.handleInitialize(request)

	if response.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", response.JSONRPC)
	}

	if response.ID != 1 {
		t.Errorf("Expected ID 1, got %v", response.ID)
	}

	if response.Error != nil {
		t.Errorf("Unexpected error: %v", response.Error)
	}

	initResponse, ok := response.Result.(InitializeResponse)
	if !ok {
		t.Error("Expected InitializeResponse result")
	}

	if initResponse.ServerInfo.Name != "test-server" {
		t.Errorf("Expected server name 'test-server', got '%s'", initResponse.ServerInfo.Name)
	}

	if initResponse.ServerInfo.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", initResponse.ServerInfo.Version)
	}
}

func TestBaseMCPServer_handleRequest_UnknownMethod(t *testing.T) {
	ctx := context.Background()
	getToolList := func() ToolList {
		return ToolList{Tools: []Tool{}}
	}

	server := NewBaseMCPServer("test-server", "1.0.0", ctx, getToolList)

	request := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "unknown_method",
		Params:  json.RawMessage(`{}`),
	}

	response := server.handleRequest(request)

	if response.Error == nil {
		t.Error("Expected error for unknown method")
	}

	if response.Error.Code != -32601 {
		t.Errorf("Expected error code -32601, got %d", response.Error.Code)
	}

	expectedMessage := "Method not found: unknown_method"
	if response.Error.Message != expectedMessage {
		t.Errorf("Expected error message '%s', got '%s'", expectedMessage, response.Error.Message)
	}
}
