package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

// mockMetrika is a stub for the metrika.Client used in unit tests.
// The real client is tested separately with httptest.
type mockServer struct {
	*Server
	out *bytes.Buffer
}

func newTestServer(t *testing.T) *mockServer {
	t.Helper()
	out := &bytes.Buffer{}
	transport := NewStdioTransport(strings.NewReader(""), out)
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))

	// We need a nil metrika client — handlers are tested separately.
	srv := &Server{
		transport: transport,
		metrika:   nil,
		logger:    logger,
		version:   "test",
		tools:     nil,
	}
	srv.tools = srv.buildToolRegistry()

	return &mockServer{Server: srv, out: out}
}

func (m *mockServer) call(method string, params any) Response {
	raw, _ := json.Marshal(params)
	id := json.RawMessage(`1`)
	req := &Request{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  method,
		Params:  raw,
	}
	return m.handleRequest(context.Background(), req)
}

// ─── tests ───────────────────────────────────────────────────────────────────

func TestInitialize(t *testing.T) {
	srv := newTestServer(t)

	params := InitializeParams{
		ProtocolVersion: protocolVersion,
		ClientInfo:      ClientInfo{Name: "test-client", Version: "1.0"},
	}
	resp := srv.call("initialize", params)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(InitializeResult)
	if !ok {
		// JSON round-trip
		b, _ := json.Marshal(resp.Result)
		var r InitializeResult
		json.Unmarshal(b, &r)
		result = r
	}

	if result.ProtocolVersion != protocolVersion {
		t.Errorf("want protocolVersion %s, got %s", protocolVersion, result.ProtocolVersion)
	}
	if result.ServerInfo.Name != serverName {
		t.Errorf("want serverName %s, got %s", serverName, result.ServerInfo.Name)
	}
	if result.Capabilities.Tools == nil {
		t.Error("expected tools capability to be set")
	}
}

func TestToolsListRequiresInit(t *testing.T) {
	srv := newTestServer(t)
	// Not initialized yet.
	resp := srv.call("tools/list", nil)
	if resp.Error == nil {
		t.Fatal("expected error for tools/list before initialize")
	}
	if resp.Error.Code != CodeInvalidRequest {
		t.Errorf("want code %d, got %d", CodeInvalidRequest, resp.Error.Code)
	}
}

func TestToolsList(t *testing.T) {
	srv := newTestServer(t)
	// Initialize first.
	srv.call("initialize", InitializeParams{
		ProtocolVersion: protocolVersion,
		ClientInfo:      ClientInfo{Name: "t", Version: "1"},
	})

	resp := srv.call("tools/list", nil)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	b, _ := json.Marshal(resp.Result)
	var result toolsListResult
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}

	if len(result.Tools) == 0 {
		t.Error("expected at least one tool")
	}

	// All tools must have a name and description.
	for _, tool := range result.Tools {
		if tool.Name == "" {
			t.Error("tool has empty name")
		}
		if tool.Description == "" {
			t.Errorf("tool %s has empty description", tool.Name)
		}
		if tool.InputSchema.Type != "object" {
			t.Errorf("tool %s inputSchema.type must be 'object'", tool.Name)
		}
	}
}

func TestUnknownMethod(t *testing.T) {
	srv := newTestServer(t)
	resp := srv.call("unknown/method", nil)
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != CodeMethodNotFound {
		t.Errorf("want code %d, got %d", CodeMethodNotFound, resp.Error.Code)
	}
}

func TestPing(t *testing.T) {
	srv := newTestServer(t)
	resp := srv.call("ping", nil)
	if resp.Error != nil {
		t.Fatalf("ping error: %v", resp.Error)
	}
}

func TestHandleNotification(t *testing.T) {
	srv := newTestServer(t)
	// notifications/initialized should not panic, just log
	srv.handleNotification(context.Background(), &Request{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	})
}
