# Тестирование

## Принципы

- Только стандартный `testing` пакет — `testify` не нужен
- `net/http/httptest` для mock API сервера
- `-race` флаг обязателен в CI
- `t.Fatal` для критических ошибок, `t.Error` для проверок

---

## Unit тесты API клиента

```go title="internal/myapi/users_test.go"
package myapi

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestGetUsers(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Проверяем Authorization header
        if got := r.Header.Get("Authorization"); got != "OAuth test-token" {
            t.Errorf("auth: want %q, got %q", "OAuth test-token", got)
        }
        // Возвращаем фиктивные данные
        json.NewEncoder(w).Encode(usersResponse{
            Users: []User{
                {ID: 1, Name: "Alice"},
                {ID: 2, Name: "Bob"},
            },
        })
    }))
    defer ts.Close()

    // WithBaseURL перенаправляет на mock сервер
    client := NewClient("test-token", WithBaseURL(ts.URL))

    users, err := client.GetUsers(context.Background())
    if err != nil {
        t.Fatalf("GetUsers() error = %v", err)
    }
    if len(users) != 2 {
        t.Errorf("len = %d, want 2", len(users))
    }
    if users[0].Name != "Alice" {
        t.Errorf("users[0].Name = %q, want Alice", users[0].Name)
    }
}
```

### Тестирование ошибок

```go
func TestGetUsers_Unauthorized(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusUnauthorized)
        w.Write([]byte(`{"error": "invalid_token"}`))
    }))
    defer ts.Close()

    _, err := NewClient("bad-token", WithBaseURL(ts.URL)).
        GetUsers(context.Background())

    if err == nil {
        t.Fatal("expected error for 401, got nil")
    }
    // Убедиться что сообщение содержит код ошибки
    if !strings.Contains(err.Error(), "401") {
        t.Errorf("error should mention 401, got: %v", err)
    }
}
```

---

## Unit тесты транспорта

```go title="internal/mcp/transport_test.go"
package mcp

import (
    "bytes"
    "errors"
    "strings"
    "testing"
)

func TestTransport_ReadRequest(t *testing.T) {
    input := `{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n"
    transport := NewStdioTransport(strings.NewReader(input), &bytes.Buffer{})

    req, err := transport.ReadRequest()
    if err != nil {
        t.Fatalf("ReadRequest() error = %v", err)
    }
    if req.Method != "ping" {
        t.Errorf("Method = %q, want ping", req.Method)
    }
}

func TestTransport_InvalidJSON(t *testing.T) {
    transport := NewStdioTransport(strings.NewReader("not-json\n"), &bytes.Buffer{})

    _, err := transport.ReadRequest()
    if err == nil {
        t.Fatal("expected parse error")
    }
    var pe *parseError
    if !errors.As(err, &pe) {
        t.Errorf("expected *parseError, got %T: %v", err, err)
    }
}

func TestTransport_EOF(t *testing.T) {
    transport := NewStdioTransport(strings.NewReader(""), &bytes.Buffer{})

    _, err := transport.ReadRequest()
    if !errors.Is(err, io.EOF) {
        t.Errorf("expected io.EOF, got %v", err)
    }
}
```

---

## E2E тест протокола

Самый надёжный способ проверить MCP сервер — прогнать реальные сообщения:

```bash title="Полный lifecycle"
go build -o bin/mcp-server ./cmd/server

{
  printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}\n'
  sleep 0.1
  printf '{"jsonrpc":"2.0","method":"notifications/initialized"}\n'
  sleep 0.1
  printf '{"jsonrpc":"2.0","id":2,"method":"tools/list"}\n'
  sleep 0.1
  printf '{"jsonrpc":"2.0","id":3,"method":"ping"}\n'
  sleep 0.3
} | ACCESS_TOKEN="$ACCESS_TOKEN" ./bin/mcp-server 2>/dev/null
```

Ожидаемый вывод:
```json
{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-11-25","capabilities":{"tools":{}},"serverInfo":{"name":"my-api-mcp","version":"dev"}}}
{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"myapi_get_users",...}]}}
{"jsonrpc":"2.0","id":3,"result":{}}
```

```bash title="Тест конкретного tool"
{
  printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}\n'
  sleep 0.1
  printf '{"jsonrpc":"2.0","method":"notifications/initialized"}\n'
  sleep 0.1
  printf '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"myapi_get_users","arguments":{}}}\n'
  sleep 0.5
} | ACCESS_TOKEN="$ACCESS_TOKEN" ./bin/mcp-server 2>/dev/null
```

---

## Команды

=== "Основные"

    ```bash
    # Все тесты
    go test ./... -v -race -timeout 60s
    
    # Один пакет
    go test ./internal/myapi/... -v
    
    # Один тест
    go test ./internal/myapi/... -run TestGetUsers -v
    ```

=== "Покрытие"

    ```bash
    go test ./... -coverprofile=coverage.out -covermode=atomic
    go tool cover -func=coverage.out | tail -1
    go tool cover -html=coverage.out -o coverage.html
    ```

=== "Makefile"

    ```bash
    make test         # go test ./... -v -race
    make test-cover   # с покрытием
    make test-mcp     # E2E тест протокола
    ```
