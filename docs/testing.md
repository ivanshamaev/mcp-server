---
layout: default
title: Тестирование
nav_order: 9
permalink: /testing
---

# Тестирование
{: .no_toc }

<details open markdown="block">
  <summary>Содержание</summary>
  {: .text-delta }
1. TOC
{:toc}
</details>

---

## Принципы

- Только стандартный `testing` пакет — `testify` не используется
- `net/http/httptest` для mock-сервера API
- `t.Fatal` для критических ошибок, `t.Error` для проверок
- `-race` флаг всегда — race detector обязателен в CI

---

## Unit тесты API клиента

Каждый метод клиента тестируется с mock HTTP сервером:

```go
// internal/myapi/users_test.go
package myapi

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestGetUsers(t *testing.T) {
    // Создаём mock сервер, который имитирует ваш API
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Проверяем что клиент шлёт правильный Authorization header
        if got := r.Header.Get("Authorization"); got != "OAuth test-token" {
            t.Errorf("auth header: want %q, got %q", "OAuth test-token", got)
        }
        // Возвращаем фиктивные данные
        if err := json.NewEncoder(w).Encode(usersResponse{
            Users: []User{
                {ID: 1, Name: "Alice"},
                {ID: 2, Name: "Bob"},
            },
        }); err != nil {
            t.Errorf("encode response: %v", err)
        }
    }))
    defer ts.Close()

    // WithBaseURL перенаправляет запросы на mock сервер
    client := NewClient("test-token", WithBaseURL(ts.URL))

    users, err := client.GetUsers(context.Background())
    if err != nil {
        t.Fatalf("GetUsers() error = %v", err)
    }
    if len(users) != 2 {
        t.Errorf("GetUsers() len = %d, want 2", len(users))
    }
    if users[0].Name != "Alice" {
        t.Errorf("GetUsers()[0].Name = %q, want Alice", users[0].Name)
    }
}
```

### Тест ошибок API

```go
func TestGetUsers_Unauthorized(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusUnauthorized)
        _, _ = w.Write([]byte(`{"error": "invalid_token"}`))
    }))
    defer ts.Close()

    client := NewClient("bad-token", WithBaseURL(ts.URL))
    _, err := client.GetUsers(context.Background())

    if err == nil {
        t.Fatal("expected error for 401, got nil")
    }
    // Проверяем что сообщение содержит код ошибки
    if !strings.Contains(err.Error(), "401") {
        t.Errorf("error should mention HTTP 401, got: %v", err)
    }
}

func TestGetUser_NotFound(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusNotFound)
        _, _ = w.Write([]byte(`{"error": "user not found"}`))
    }))
    defer ts.Close()

    client := NewClient("tok", WithBaseURL(ts.URL))
    _, err := client.GetUser(context.Background(), "999")

    if err == nil {
        t.Fatal("expected error for 404, got nil")
    }
}
```

---

## Unit тесты транспорта

```go
// internal/mcp/transport_test.go
package mcp

import (
    "bytes"
    "encoding/json"
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

func TestTransport_WriteResponse(t *testing.T) {
    var buf bytes.Buffer
    transport := NewStdioTransport(strings.NewReader(""), &buf)

    id := json.RawMessage(`1`)
    resp := okResponse(&id, map[string]string{"status": "ok"})
    if err := transport.WriteResponse(resp); err != nil {
        t.Fatalf("WriteResponse() error = %v", err)
    }

    // json.Encoder добавляет \n — проверяем что есть
    if !strings.HasSuffix(buf.String(), "\n") {
        t.Error("response should end with newline")
    }
}
```

---

## Unit тесты сервера

```go
// internal/mcp/server_test.go
package mcp

import (
    "bytes"
    "strings"
    "testing"
)

// initializeServer отправляет initialize + notifications/initialized через транспорт
func initializeServer(t *testing.T, transport *StdioTransport) {
    t.Helper()
    // Фактически флаг initialized устанавливается при handleInitialize
}

func TestServer_Ping(t *testing.T) {
    // Создаём сервер с nil клиентом — для ping не нужен
    input := strings.Join([]string{
        `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`,
        `{"jsonrpc":"2.0","method":"notifications/initialized"}`,
        `{"jsonrpc":"2.0","id":2,"method":"ping"}`,
    }, "\n") + "\n"

    var out bytes.Buffer
    transport := NewStdioTransport(strings.NewReader(input), &out)
    server := NewServer(transport, nil, slog.Default(), "test")

    ctx, cancel := context.WithCancel(context.Background())
    cancel() // не ждём EOF — запускаем с отменённым контекстом

    // Но нам нужно обработать запросы до отмены...
    // Лучше использовать goroutine:
    done := make(chan error, 1)
    go func() { done <- server.Run(context.Background()) }()

    // Дать серверу обработать сообщения
    time.Sleep(50 * time.Millisecond)

    // Проверить ответы в out
    responses := strings.Split(strings.TrimSpace(out.String()), "\n")
    // responses[0] — ответ на initialize
    // responses[1] — ответ на ping
}
```

{: .note }
> Тесты сервера сложнее из-за его асинхронной природы. На практике большую ценность дают тесты транспорта и клиента, а сервер покрывается E2E тестом.

---

## E2E тест протокола

Самый надёжный способ проверить что сервер работает — прогнать через него реальные MCP сообщения:

```bash
# Собрать
export PATH="$HOME/.local/go/bin:$PATH"
go build -o bin/mcp-server ./cmd/server

# E2E тест
{
  printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}\n'
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
{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{}},"serverInfo":{"name":"my-api-mcp","version":"dev"}}}
{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"myapi_get_users",...},...]}}
{"jsonrpc":"2.0","id":3,"result":{}}
```

### Тест конкретного tool

```bash
{
  printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}\n'
  sleep 0.1
  printf '{"jsonrpc":"2.0","method":"notifications/initialized"}\n'
  sleep 0.1
  printf '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"myapi_get_users","arguments":{}}}\n'
  sleep 0.5
} | ACCESS_TOKEN="$ACCESS_TOKEN" ./bin/mcp-server 2>/dev/null
```

---

## Команды запуска тестов

```bash
# Все тесты с verbose и race detector
go test ./... -v -race -timeout 60s

# Только определённый пакет
go test ./internal/myapi/... -v

# Один тест
go test ./internal/myapi/... -run TestGetUsers -v

# Покрытие
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
go tool cover -html=coverage.out -o coverage.html

# В Makefile (если создали)
make test
make test-cover
make test-mcp   # E2E тест (требует ACCESS_TOKEN)
```

---

## Что дальше?

- [CI/CD и релизы]({{ site.baseurl }}/cicd) — автоматизация сборки и выпуска
