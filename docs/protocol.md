---
layout: default
title: Протокол и транспорт
nav_order: 5
permalink: /protocol
---

# Протокол и транспорт
{: .no_toc }

<details open markdown="block">
  <summary>Содержание</summary>
  {: .text-delta }
1. TOC
{:toc}
</details>

---

## Типы JSON-RPC (`internal/mcp/types.go`)

Начнём с определения всех типов. Этот файл одинаков для любого MCP сервера — копируйте as-is.

```go
package mcp

import "encoding/json"

// ─── JSON-RPC 2.0 ─────────────────────────────────────────────────────────────

// Request — входящее сообщение (запрос или уведомление).
// Уведомления имеют ID == nil.
type Request struct {
    JSONRPC string           `json:"jsonrpc"`
    ID      *json.RawMessage `json:"id,omitempty"`
    Method  string           `json:"method"`
    Params  json.RawMessage  `json:"params,omitempty"`
}

// Response — исходящий ответ.
type Response struct {
    JSONRPC string           `json:"jsonrpc"`
    ID      *json.RawMessage `json:"id"`
    Result  any              `json:"result,omitempty"`
    Error   *RPCError        `json:"error,omitempty"`
}

// RPCError — объект ошибки JSON-RPC 2.0.
type RPCError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Data    any    `json:"data,omitempty"`
}

// Стандартные коды ошибок JSON-RPC 2.0.
const (
    CodeParseError     = -32700  // невалидный JSON
    CodeInvalidRequest = -32600  // нарушена структура JSON-RPC
    CodeMethodNotFound = -32601  // неизвестный метод
    CodeInvalidParams  = -32602  // неверные параметры
    CodeInternalError  = -32603  // внутренняя ошибка сервера
)
```

### Почему `*json.RawMessage` для ID?

ID в JSON-RPC может быть строкой, числом или `null`. `*json.RawMessage` позволяет:
1. Передать ID клиенту обратно без изменений
2. Различить `null` (явный null) и отсутствие поля (уведомление)

```go
// Ответ на запрос
func okResponse(id *json.RawMessage, result any) Response {
    return Response{JSONRPC: "2.0", ID: id, Result: result}
}

// Ответ с ошибкой
func errorResponse(id *json.RawMessage, code int, msg string) Response {
    return Response{JSONRPC: "2.0", ID: id, Error: &RPCError{Code: code, Message: msg}}
}
```

### MCP-специфичные типы

```go
// Результат вызова инструмента
type ToolCallResult struct {
    Content []ContentItem `json:"content"`
    IsError bool          `json:"isError,omitempty"`
}

// Единица контента (текст, изображение, ресурс)
type ContentItem struct {
    Type string `json:"type"` // "text" | "image" | "resource"
    Text string `json:"text,omitempty"`
}

// Параметры tools/call
type ToolCallParams struct {
    Name      string         `json:"name"`
    Arguments map[string]any `json:"arguments"`
}

// Описание инструмента (для tools/list)
type Tool struct {
    Name        string      `json:"name"`
    Description string      `json:"description"`
    InputSchema InputSchema `json:"inputSchema"`
}

// JSON Schema для входных параметров инструмента
type InputSchema struct {
    Type       string              `json:"type"` // всегда "object"
    Properties map[string]Property `json:"properties"`
    Required   []string            `json:"required,omitempty"`
}

type Property struct {
    Type        string   `json:"type"`
    Description string   `json:"description"`
    Enum        []string `json:"enum,omitempty"`
    Default     any      `json:"default,omitempty"`
}
```

---

## Транспорт (`internal/mcp/transport.go`)

Транспорт — это тонкий слой между stdio и JSON-RPC. Одинаков для любого MCP сервера.

```go
package mcp

import (
    "bufio"
    "encoding/json"
    "fmt"
    "io"
    "sync"
)

type StdioTransport struct {
    scanner *bufio.Scanner
    encoder *json.Encoder
    mu      sync.Mutex  // защита encoder от конкурентной записи
}

func NewStdioTransport(r io.Reader, w io.Writer) *StdioTransport {
    scanner := bufio.NewScanner(r)

    // 4 MB буфер — нужен для больших ответов (Logs API, большие JSON)
    const maxTokenSize = 4 * 1024 * 1024
    scanner.Buffer(make([]byte, maxTokenSize), maxTokenSize)

    return &StdioTransport{
        scanner: scanner,
        encoder: json.NewEncoder(w),
    }
}
```

### Чтение запросов

```go
func (t *StdioTransport) ReadRequest() (*Request, error) {
    if !t.scanner.Scan() {
        if err := t.scanner.Err(); err != nil {
            return nil, fmt.Errorf("stdin read: %w", err)
        }
        return nil, io.EOF  // нормальное завершение
    }

    var req Request
    if err := json.Unmarshal(t.scanner.Bytes(), &req); err != nil {
        return nil, &parseError{raw: t.scanner.Text(), err: err}
    }
    if req.JSONRPC != "2.0" {
        return nil, &invalidRequestError{msg: `jsonrpc must be "2.0"`}
    }
    return &req, nil
}
```

{: .important }
> `bufio.Scanner` читает построчно. Каждый JSON-RPC message — одна строка. Это стандарт MCP stdio транспорта.

### Запись ответов

```go
func (t *StdioTransport) WriteResponse(resp Response) error {
    t.mu.Lock()
    defer t.mu.Unlock()
    return t.encoder.Encode(resp)  // Encode добавляет \n автоматически
}
```

Mutex нужен если вы когда-нибудь захотите отправлять ответы из goroutine.

### Ошибки транспорта

```go
// Невалидный JSON
type parseError struct {
    raw string
    err error
}
func (e *parseError) Error() string {
    return fmt.Sprintf("parse error: %v (raw: %.80s)", e.err, e.raw)
}

// Нарушена структура JSON-RPC
type invalidRequestError struct{ msg string }
func (e *invalidRequestError) Error() string { return "invalid request: " + e.msg }
```

---

## Размер буфера: почему 4 MB?

По умолчанию `bufio.Scanner` имеет буфер 64 KB. Это мало для:
- Больших ответов API с тысячами записей
- Logs API (TSV данные могут быть мегабайтами)
- Списков с деталями по каждому объекту

Симптом превышения буфера: `bufio.Scanner: token too long`

{: .note }
> 4 MB — достаточно для практически любого одиночного JSON ответа. Для потоковых данных (если API возвращает гигабайты) нужен другой подход — разбивайте на части на уровне API клиента.

---

## Обработка ошибок протокола vs ошибок tool

Разница критична для правильной работы клиента:

```
Сценарий 1: Неизвестный метод
─────────────────────────────
→ {"method": "unknown/method", "id": 1}
← {"id": 1, "error": {"code": -32601, "message": "method not found"}}
   ^^^^^^^^
   Это protocol error — клиент знает что метод не существует

Сценарий 2: API вернул 404
──────────────────────────
→ {"method": "tools/call", "params": {"name": "myapi_get_user", ...}}
← {"id": 1, "result": {"content": [{"type": "text", "text": "User not found: 404"}], "isError": true}}
                        ^^^^^^^^^
                        Это tool error — протокол работает, но инструмент вернул ошибку
```

Правило: **никогда не используйте JSON-RPC `error` для ошибок бизнес-логики**.

---

## Что дальше?

- [Реализация сервера]({{ site.baseurl }}/server) — Server struct и main loop
