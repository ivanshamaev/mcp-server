package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// StdioTransport implements the MCP stdio transport:
// - Reads newline-delimited JSON-RPC messages from a reader (os.Stdin).
// - Writes newline-delimited JSON-RPC messages to a writer (os.Stdout).
//
// Writing is protected by a mutex so that concurrent goroutines can safely
// send responses.
type StdioTransport struct {
	scanner *bufio.Scanner
	encoder *json.Encoder
	mu      sync.Mutex
}

// NewStdioTransport creates a new stdio transport.
// bufSize controls the scanner buffer (default 1 MB is enough for most payloads).
func NewStdioTransport(r io.Reader, w io.Writer) *StdioTransport {
	scanner := bufio.NewScanner(r)
	// Allow lines up to 4 MB (large log downloads can be chunky).
	const maxTokenSize = 4 * 1024 * 1024
	scanner.Buffer(make([]byte, maxTokenSize), maxTokenSize)

	return &StdioTransport{
		scanner: scanner,
		encoder: json.NewEncoder(w),
	}
}

// ReadRequest reads and parses the next JSON-RPC message from stdin.
// Returns io.EOF when the input stream is closed.
func (t *StdioTransport) ReadRequest() (*Request, error) {
	if !t.scanner.Scan() {
		if err := t.scanner.Err(); err != nil {
			return nil, fmt.Errorf("stdin read: %w", err)
		}
		return nil, io.EOF
	}

	var req Request
	if err := json.Unmarshal(t.scanner.Bytes(), &req); err != nil {
		return nil, &parseError{raw: t.scanner.Text(), err: err}
	}
	if req.JSONRPC != "2.0" {
		return nil, &invalidRequestError{msg: "jsonrpc must be \"2.0\""}
	}
	return &req, nil
}

// WriteResponse serialises and writes a JSON-RPC response to stdout.
// The encoder appends a newline automatically.
func (t *StdioTransport) WriteResponse(resp Response) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.encoder.Encode(resp)
}

// ─── transport-level error types ─────────────────────────────────────────────

type parseError struct {
	raw string
	err error
}

func (e *parseError) Error() string { return fmt.Sprintf("parse error: %v (raw: %.80s)", e.err, e.raw) }

type invalidRequestError struct{ msg string }

func (e *invalidRequestError) Error() string { return "invalid request: " + e.msg }
