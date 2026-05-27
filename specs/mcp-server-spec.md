# MCP Server Go — Спецификация

## Протокол MCP

**Model Context Protocol** (MCP) — открытый стандарт на базе **JSON-RPC 2.0**.
Спецификация: https://modelcontextprotocol.io/specification/latest

### Версия протокола

Поддерживаемая версия: **`2024-11-05`** (стабильная, поддерживается всеми клиентами).

## Транспорт: Stdio

Для локального MCP-сервера используется **stdio транспорт**:

```
opencode/Claude Code → [stdin] → MCP Server → [stdout] → opencode/Claude Code
                                     ↓
                                  [stderr] → логи
```

**Формат фреймирования:** Каждое JSON-RPC сообщение — отдельная строка (`\n`-terminated).
Клиент читает по строкам, сервер отвечает одной строкой на запрос.

```go
// Основной цикл чтения
scanner := bufio.NewScanner(os.Stdin)
scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB буфер
for scanner.Scan() {
    line := scanner.Bytes()
    // обработка JSON-RPC запроса
}
```

## Жизненный цикл

```
Client                          Server
  │                               │
  │── initialize ──────────────►  │  (1) Клиент инициирует соединение
  │  ◄─────────────── result ───  │  (2) Сервер отвечает capabilities
  │── notifications/initialized ► │  (3) Клиент сигнализирует готовность
  │                               │
  │── tools/list ───────────────► │  (4) Клиент запрашивает список tools
  │  ◄─────────────── result ───  │
  │                               │
  │── tools/call ───────────────► │  (5) Клиент вызывает tool
  │  ◄─────────────── result ───  │
  │                               │
  │── [EOF / SIGTERM] ──────────  │  (6) Завершение
```

## JSON-RPC 2.0 Типы

```go
// Базовые типы JSON-RPC 2.0
type Request struct {
    JSONRPC string          `json:"jsonrpc"`           // всегда "2.0"
    ID      *json.RawMessage `json:"id,omitempty"`    // null для notifications
    Method  string          `json:"method"`
    Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
    JSONRPC string           `json:"jsonrpc"` // всегда "2.0"
    ID      *json.RawMessage `json:"id"`
    Result  any              `json:"result,omitempty"`
    Error   *RPCError        `json:"error,omitempty"`
}

type Notification struct {
    JSONRPC string `json:"jsonrpc"` // всегда "2.0"
    Method  string `json:"method"`
    Params  any    `json:"params,omitempty"`
}

type RPCError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Data    any    `json:"data,omitempty"`
}

// Стандартные коды ошибок JSON-RPC
const (
    CodeParseError     = -32700 // Invalid JSON
    CodeInvalidRequest = -32600 // Not valid JSON-RPC
    CodeMethodNotFound = -32601 // Method doesn't exist
    CodeInvalidParams  = -32602 // Invalid method params
    CodeInternalError  = -32603 // Internal server error
)
```

## Метод: initialize

**Запрос клиента:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2024-11-05",
    "capabilities": {},
    "clientInfo": {
      "name": "opencode",
      "version": "0.1.0"
    }
  }
}
```

**Ответ сервера:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2024-11-05",
    "capabilities": {
      "tools": {}
    },
    "serverInfo": {
      "name": "yandex-metrika-mcp",
      "version": "1.0.0"
    }
  }
}
```

```go
type InitializeParams struct {
    ProtocolVersion string       `json:"protocolVersion"`
    Capabilities    Capabilities `json:"capabilities"`
    ClientInfo      ClientInfo   `json:"clientInfo"`
}

type InitializeResult struct {
    ProtocolVersion string       `json:"protocolVersion"`
    Capabilities    Capabilities `json:"capabilities"`
    ServerInfo      ServerInfo   `json:"serverInfo"`
}

type Capabilities struct {
    Tools     *ToolsCapability     `json:"tools,omitempty"`
    Resources *ResourcesCapability `json:"resources,omitempty"`
}

type ToolsCapability struct {
    ListChanged bool `json:"listChanged,omitempty"`
}
```

## Метод: tools/list

**Запрос:**
```json
{"jsonrpc": "2.0", "id": 2, "method": "tools/list"}
```

**Ответ:**
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "tools": [
      {
        "name": "metrika_get_counters",
        "description": "Получить список всех счётчиков Yandex Metrika",
        "inputSchema": {
          "type": "object",
          "properties": {},
          "required": []
        }
      }
    ]
  }
}
```

```go
type Tool struct {
    Name        string     `json:"name"`
    Description string     `json:"description"`
    InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
    Type       string              `json:"type"`        // всегда "object"
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

## Метод: tools/call

**Запрос:**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "metrika_get_report",
    "arguments": {
      "counter_id": "12345678",
      "metrics": "ym:s:visits,ym:s:pageviews",
      "dimensions": "ym:s:date",
      "date1": "2024-01-01",
      "date2": "2024-01-31"
    }
  }
}
```

**Ответ (успех):**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "{ ... JSON данные от Metrika API ... }"
      }
    ],
    "isError": false
  }
}
```

**Ответ (ошибка в tool, не протокольная):**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Error: counter 12345678 not found (HTTP 404)"
      }
    ],
    "isError": true
  }
}
```

> **Важно:** Ошибки выполнения tool (API ошибки, неверные параметры) возвращаются
> через `result.isError: true`, а не через `error`. JSON-RPC `error` только для
> протокольных ошибок (неверный метод, неверный JSON и т.д.)

```go
type ToolCallParams struct {
    Name      string         `json:"name"`
    Arguments map[string]any `json:"arguments"`
}

type ToolCallResult struct {
    Content []ContentItem `json:"content"`
    IsError bool          `json:"isError,omitempty"`
}

type ContentItem struct {
    Type string `json:"type"` // "text" | "image" | "resource"
    Text string `json:"text,omitempty"`
}
```

## Реестр Tools: Yandex Metrika

### `metrika_get_counters`
Получить список счётчиков пользователя.
```json
{ "properties": {}, "required": [] }
```
→ Вызов: `GET /management/v1/counters`

---

### `metrika_get_counter`
Информация о конкретном счётчике.
```json
{
  "properties": {
    "counter_id": { "type": "string", "description": "ID счётчика" }
  },
  "required": ["counter_id"]
}
```
→ Вызов: `GET /management/v1/counter/{counter_id}`

---

### `metrika_get_report`
Получить статистический отчёт.
```json
{
  "properties": {
    "counter_id": { "type": "string", "description": "ID счётчика" },
    "metrics":    { "type": "string", "description": "Метрики через запятую, напр. ym:s:visits,ym:s:pageviews" },
    "dimensions": { "type": "string", "description": "Измерения через запятую, напр. ym:s:date" },
    "date1":      { "type": "string", "description": "Начало периода YYYY-MM-DD (default: 7daysAgo)" },
    "date2":      { "type": "string", "description": "Конец периода YYYY-MM-DD (default: today)" },
    "sort":       { "type": "string", "description": "Поле сортировки" },
    "limit":      { "type": "string", "description": "Лимит строк (default: 100)" },
    "filters":    { "type": "string", "description": "Фильтры в формате Metrika" }
  },
  "required": ["counter_id", "metrics"]
}
```
→ Вызов: `GET /stat/v1/data`

---

### `metrika_get_goals`
Список целей счётчика.
```json
{
  "properties": {
    "counter_id": { "type": "string", "description": "ID счётчика" }
  },
  "required": ["counter_id"]
}
```
→ Вызов: `GET /management/v1/counter/{counter_id}/goals`

---

### `metrika_get_segments`
Список сегментов счётчика.
```json
{
  "properties": {
    "counter_id": { "type": "string", "description": "ID счётчика" }
  },
  "required": ["counter_id"]
}
```
→ Вызов: `GET /management/v1/counter/{counter_id}/segments`

---

### `metrika_list_logs`
Список запросов на выгрузку логов.
```json
{
  "properties": {
    "counter_id": { "type": "string", "description": "ID счётчика" }
  },
  "required": ["counter_id"]
}
```
→ Вызов: `GET /logs/v1/counter/{counter_id}/logrequests`

---

### `metrika_create_log_request`
Создать запрос на выгрузку логов.
```json
{
  "properties": {
    "counter_id": { "type": "string", "description": "ID счётчика" },
    "fields":     { "type": "string", "description": "Поля через запятую" },
    "source":     { "type": "string", "description": "visits или hits", "enum": ["visits", "hits"] },
    "date1":      { "type": "string", "description": "Начало периода YYYY-MM-DD" },
    "date2":      { "type": "string", "description": "Конец периода YYYY-MM-DD" }
  },
  "required": ["counter_id", "fields", "source", "date1", "date2"]
}
```
→ Вызов: `POST /logs/v1/counter/{counter_id}/logrequests`

---

### `metrika_download_log`
Скачать часть выгруженных логов.
```json
{
  "properties": {
    "counter_id":  { "type": "string", "description": "ID счётчика" },
    "request_id":  { "type": "string", "description": "ID запроса логов" },
    "part_number": { "type": "string", "description": "Номер части (default: 0)" }
  },
  "required": ["counter_id", "request_id"]
}
```
→ Вызов: `GET /logs/v1/counter/{counter_id}/logrequest/{request_id}/part/{part}/download`

## Архитектура Go кода

```
main.go
  └── config.Load()           // читает env vars / .env файл
  └── metrika.NewClient(cfg)  // HTTP клиент к Yandex API
  └── mcp.NewServer(client)   // MCP сервер с зарегистрированными tools
  └── server.Run(ctx)         // запуск stdio транспорта

mcp.Server
  ├── transport: StdioTransport   // bufio.Scanner по stdin
  ├── tools: []Tool               // статический реестр tools
  ├── initialized: bool           // флаг после initialize handshake
  └── Handle(ctx, req) Response   // роутер методов

StdioTransport
  ├── reader: *bufio.Scanner      // stdin
  ├── writer: *json.Encoder       // stdout (с мьютексом!)
  └── ReadRequest() / WriteResponse()

MetrikaClient
  ├── baseURL: string
  ├── token: string
  ├── httpClient: *http.Client
  └── Methods: GetCounters, GetReport, GetGoals, ...
```

## Важные детали реализации

### Thread safety для writer

```go
type StdioTransport struct {
    reader  *bufio.Scanner
    encoder *json.Encoder
    mu      sync.Mutex // защита записи в stdout
}

func (t *StdioTransport) WriteResponse(resp Response) error {
    t.mu.Lock()
    defer t.mu.Unlock()
    return t.encoder.Encode(resp) // Encode добавляет \n
}
```

### Обработка неизвестных методов

```go
// Согласно MCP spec, неизвестные методы → MethodNotFound
// НО: notifications (без id) просто игнорируются
if req.ID == nil {
    // это notification — игнорируем тихо
    continue
}
return RPCError{Code: CodeMethodNotFound, Message: "method not found: " + req.Method}
```

### Graceful shutdown

```go
ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer cancel()

// Когда stdin закрывается (EOF) — завершаем работу
// Когда сигнал — завершаем работу
```

### Формат ответа Metrika в content

Данные от Yandex Metrika API возвращаем как prettified JSON в `text` поле:

```go
func formatJSONResponse(v any) (string, error) {
    b, err := json.MarshalIndent(v, "", "  ")
    if err != nil {
        return "", err
    }
    return string(b), nil
}
```

## Тестирование MCP протокола

### Unit тесты

```go
// Тест initialize handshake
func TestInitialize(t *testing.T) {
    client := metrika.NewMockClient()
    srv := mcp.NewServer(client)

    req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`
    resp := srv.HandleRaw([]byte(req))

    var result map[string]any
    require.NoError(t, json.Unmarshal(resp, &result))
    assert.Equal(t, "2024-11-05", result["result"].(map[string]any)["protocolVersion"])
}
```

### Integration тест (E2E)

```bash
# Последовательность запросов через pipe
{
  echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}'
  echo '{"jsonrpc":"2.0","method":"notifications/initialized"}'
  echo '{"jsonrpc":"2.0","id":2,"method":"tools/list"}'
  echo '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"metrika_get_counters","arguments":{}}}'
} | ACCESS_TOKEN=$ACCESS_TOKEN ./bin/mcp-server
```

### MCP Inspector

```bash
npx @modelcontextprotocol/inspector ./bin/mcp-server
# Открывает веб-UI на http://localhost:5173
```
