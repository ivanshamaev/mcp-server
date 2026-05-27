package mcp

import "encoding/json"

// ─── JSON-RPC 2.0 base types ─────────────────────────────────────────────────

// Request represents an incoming JSON-RPC 2.0 request or notification.
// Notifications have ID == nil.
type Request struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

// Response represents an outgoing JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Result  any              `json:"result,omitempty"`
	Error   *RPCError        `json:"error,omitempty"`
}

// Notification is a one-way JSON-RPC 2.0 message with no id.
type Notification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// RPCError represents a JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *RPCError) Error() string { return e.Message }

// Standard JSON-RPC 2.0 error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

func errorResponse(id *json.RawMessage, code int, msg string) Response {
	return Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RPCError{Code: code, Message: msg},
	}
}

func okResponse(id *json.RawMessage, result any) Response {
	return Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

// ─── MCP Protocol types ───────────────────────────────────────────────────────

// InitializeParams is the params of the initialize request.
type InitializeParams struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      ClientInfo         `json:"clientInfo"`
}

// InitializeResult is the result of the initialize request.
// Instructions field added in MCP 2025-11-25.
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
	Instructions    string             `json:"instructions,omitempty"` // added in 2025-11-25
}

// ClientInfo describes the MCP client.
// Fields added in MCP 2025-11-25: Title, Description.
type ClientInfo struct {
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

// ServerInfo describes this MCP server.
// Fields added in MCP 2025-11-25: Title, Description.
type ServerInfo struct {
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

// ClientCapabilities lists what the client supports.
// ElicitationCapability expanded in MCP 2025-11-25 (url sub-capability).
type ClientCapabilities struct {
	Elicitation *ElicitationCapability `json:"elicitation,omitempty"`
}

// ElicitationCapability describes client-side elicitation support.
// Introduced in MCP 2024-11-05, url sub-capability added in 2025-11-25.
type ElicitationCapability struct {
	Form *struct{} `json:"form,omitempty"`
	URL  *struct{} `json:"url,omitempty"` // added in 2025-11-25
}

// ServerCapabilities lists what this server supports.
type ServerCapabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

// ToolsCapability signals support for the tools primitive.
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ─── Tools types ──────────────────────────────────────────────────────────────

// Tool describes an MCP tool available to call.
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

// InputSchema is a simplified JSON Schema for tool input.
type InputSchema struct {
	Type       string              `json:"type"` // always "object"
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

// Property is a single property in an InputSchema.
type Property struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
	Default     any      `json:"default,omitempty"`
}

// ToolCallParams is the params of a tools/call request.
type ToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// ToolCallResult is the result of a tools/call request.
type ToolCallResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ContentItem is a single piece of content in a ToolCallResult.
type ContentItem struct {
	Type string `json:"type"` // "text" | "image" | "resource"
	Text string `json:"text,omitempty"`
}

// toolsListResult is the result of tools/list.
type toolsListResult struct {
	Tools []Tool `json:"tools"`
}

// textContent returns a ToolCallResult with a single text content item.
func textContent(text string) ToolCallResult {
	return ToolCallResult{Content: []ContentItem{{Type: "text", Text: text}}}
}

// errorContent returns a ToolCallResult signalling a tool-level error.
func errorContent(text string) ToolCallResult {
	return ToolCallResult{
		Content: []ContentItem{{Type: "text", Text: text}},
		IsError: true,
	}
}
