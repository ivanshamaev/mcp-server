package mcp

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestReadRequest_Valid(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"
	tr := NewStdioTransport(strings.NewReader(input), &bytes.Buffer{})

	req, err := tr.ReadRequest()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Method != "tools/list" {
		t.Errorf("want method tools/list, got %s", req.Method)
	}
}

func TestReadRequest_InvalidJSON(t *testing.T) {
	input := "not json\n"
	tr := NewStdioTransport(strings.NewReader(input), &bytes.Buffer{})

	_, err := tr.ReadRequest()
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestReadRequest_EOF(t *testing.T) {
	tr := NewStdioTransport(strings.NewReader(""), &bytes.Buffer{})

	_, err := tr.ReadRequest()
	if err != io.EOF {
		t.Errorf("want io.EOF, got %v", err)
	}
}

func TestWriteResponse(t *testing.T) {
	var out bytes.Buffer
	tr := NewStdioTransport(strings.NewReader(""), &out)

	id := json.RawMessage(`42`)
	resp := okResponse(&id, map[string]string{"hello": "world"})
	if err := tr.WriteResponse(resp); err != nil {
		t.Fatalf("write error: %v", err)
	}

	var decoded Response
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("decode written response: %v", err)
	}
	if decoded.JSONRPC != "2.0" {
		t.Errorf("want jsonrpc 2.0, got %s", decoded.JSONRPC)
	}
}

func TestReadRequest_Notification(t *testing.T) {
	// Notification has no id — ID field should be nil.
	input := `{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n"
	tr := NewStdioTransport(strings.NewReader(input), &bytes.Buffer{})

	req, err := tr.ReadRequest()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.ID != nil {
		t.Error("notification should have nil ID")
	}
}
