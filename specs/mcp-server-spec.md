# MCP Server Go — Спецификация

## Протокол MCP

**Model Context Protocol** (MCP) — открытый стандарт на базе **JSON-RPC 2.0**.
Спецификация: https://modelcontextprotocol.io/specification/latest

### Версия протокола

Поддерживаемая версия: **`2025-11-25`** (актуальная стабильная).

История версий: `2024-11-05` → `2025-03-26` → `2025-06-18` → **`2025-11-25`**

Изменения в `2025-11-25` относительно `2024-11-05`:
- `ClientInfo` / `ServerInfo`: новые опциональные поля `title`, `description`
- `InitializeResult`: новое опциональное поле `instructions`
- `ClientCapabilities.Elicitation`: расширена — добавлено подполе `url` (URL-based elicitation)
- Уточнение: tool validation errors → `result.isError: true` (не protocol error)
- Уточнение: stdio транспорт — все типы логов разрешены в `stderr`

## Транспорт: Stdio

Для локального MCP-сервера используется **stdio транспорт**:

```
opencode/Claude Code
    │  stdin (JSON-RPC запросы, по одному на строку)
    ▼
MCP Server (./bin/mcp-server или yametrika-mcp)
    │  stdout (JSON-RPC ответы, по одному на строку)
    ▼
opencode/Claude Code
    │
    ▼  stderr → логи (JSON, slog)
```

**Формат фреймирования:** каждое JSON-RPC сообщение — одна строка, завершённая `\n`.
`json.Encoder.Encode()` добавляет `\n` автоматически.

### Scanner buffer

Буфер `bufio.Scanner` для stdin установлен в **4 MB** (не 1 MB — логи Metrika могут быть большими):

```go
const maxTokenSize = 4 * 1024 * 1024 // 4 MB
scanner.Buffer(make([]byte, maxTokenSize), maxTokenSize)
```

## Жизненный цикл

```
Client                          Server
  │                               │
  │── initialize ──────────────►  │  (1) Инициация + согласование capabilities
  │  ◄─────────────── result ───  │  (2) Сервер отвечает своими capabilities
  │── notifications/initialized ► │  (3) Клиент сигнализирует готовность (без ответа)
  │                               │
  │── tools/list ───────────────► │  (4) Клиент запрашивает список tools
  │  ◄─────────────── result ───  │
  │                               │
  │── tools/call ───────────────► │  (5) Клиент вызывает tool
  │  ◄─────────────── result ───  │
  │                               │
  │── ping ─────────────────────► │  (6) Keepalive (опционально)
  │  ◄─────────────── result ───  │
  │                               │
  │── [EOF / SIGTERM] ──────────► │  (7) Завершение
```

**Завершение:**
- EOF в stdin → сервер завершает работу штатно
- SIGTERM / SIGINT → graceful shutdown через `signal.NotifyContext`

## JSON-RPC 2.0 Типы

```go
type Request struct {
    JSONRPC string           `json:"jsonrpc"`          // всегда "2.0"
    ID      *json.RawMessage `json:"id,omitempty"`     // nil = notification
    Method  string           `json:"method"`
    Params  json.RawMessage  `json:"params,omitempty"`
}

type Response struct {
    JSONRPC string           `json:"jsonrpc"` // всегда "2.0"
    ID      *json.RawMessage `json:"id"`
    Result  any              `json:"result,omitempty"`
    Error   *RPCError        `json:"error,omitempty"`
}

type RPCError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Data    any    `json:"data,omitempty"`
}

const (
    CodeParseError     = -32700 // Invalid JSON
    CodeInvalidRequest = -32600 // Not valid JSON-RPC 2.0
    CodeMethodNotFound = -32601 // Unknown method
    CodeInvalidParams  = -32602 // Bad params
    CodeInternalError  = -32603 // Server error
)
```

### Notifications (без id)

Уведомления от клиента **не требуют ответа**. Сервер их принимает и игнорирует (кроме известных):

```go
if req.ID == nil {
    // notification — не отвечаем
    handleNotification(req)
    continue
}
```

Известные notifications:
- `notifications/initialized` — логируется как INFO

## Поддерживаемые методы

| Метод | Требует initialize? | Описание |
|-------|:-----------------:|----------|
| `initialize` | ❌ | Handshake, согласование capabilities |
| `notifications/initialized` | ❌ | Notification от клиента (нет ответа) |
| `tools/list` | ✅ | Список доступных tools |
| `tools/call` | ✅ | Вызов tool |
| `ping` | ❌ | Keepalive → `{}` |

Любой другой метод → `{"code": -32601, "message": "method not found: <method>"}`.

## Метод: initialize

```json
// Запрос
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2024-11-05",
    "capabilities": {},
    "clientInfo": { "name": "opencode", "version": "0.1.0" }
  }
}

// Ответ
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
      "version": "1.2.0"
    }
  }
}
```

Повторный `initialize` допустим (идемпотентен) — просто обновляет флаг и логирует.

## Метод: tools/list

```json
// Запрос
{ "jsonrpc": "2.0", "id": 2, "method": "tools/list" }

// Ответ
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "tools": [
      {
        "name": "metrika_get_counters",
        "description": "Получить список всех счётчиков Yandex Metrika аккаунта",
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

Список tools статический (определяется при старте сервера, не меняется в runtime).

## Метод: tools/call

```json
// Запрос
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "metrika_get_report",
    "arguments": {
      "counter_id": "96819956",
      "metrics": "ym:s:visits,ym:s:pageviews",
      "dimensions": "ym:s:date",
      "date1": "7daysAgo",
      "date2": "today"
    }
  }
}

// Ответ (успех)
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "content": [{ "type": "text", "text": "{ ... prettified JSON ... }" }],
    "isError": false
  }
}

// Ответ (ошибка на уровне tool — API недоступен, неверный counter_id и т.д.)
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "content": [{ "type": "text", "text": "Ошибка получения отчёта: HTTP 404 ..." }],
    "isError": true
  }
}
```

> **Ключевое правило:** Ошибки выполнения tool (недоступный API, неверные параметры) → `result.isError: true`.
> JSON-RPC `error` только для протокольных сбоев (неверный JSON, неизвестный метод).

## Реестр Tools: Yandex Metrika

### `metrika_get_counters`
Список всех счётчиков аккаунта.
- Параметры: нет
- API: `GET /management/v1/counters?per_page=1000`

---

### `metrika_get_counter`
Подробная информация о счётчике.
- Параметры: `counter_id` (string, required)
- API: `GET /management/v1/counter/{counter_id}`

---

### `metrika_get_report`
Статистический отчёт (Reports API).
- Параметры:

| Параметр | Тип | Обязательный | Default | Описание |
|----------|-----|:---:|---------|----------|
| `counter_id` | string | ✅ | — | ID счётчика |
| `metrics` | string | ✅ | — | Метрики через запятую: `ym:s:visits,ym:s:pageviews` |
| `dimensions` | string | ❌ | — | Измерения: `ym:s:date,ym:s:sourceEngine` |
| `date1` | string | ❌ | `7daysAgo` | Начало: `YYYY-MM-DD` или `7daysAgo`, `today` |
| `date2` | string | ❌ | `today` | Конец периода |
| `sort` | string | ❌ | — | Сортировка: `-ym:s:visits` |
| `limit` | string | ❌ | `100` | Лимит строк (1–100000) |
| `filters` | string | ❌ | — | Фильтры Metrika: `ym:s:regionCity=='Москва'` |
| `group` | string | ❌ | — | Группировка: `day`, `week`, `month` |

- API: `GET /stat/v1/data`

---

### `metrika_get_goals`
Список целей счётчика.
- Параметры: `counter_id` (string, required)
- API: `GET /management/v1/counter/{counter_id}/goals`

---

### `metrika_get_segments`
Список сегментов счётчика.
- Параметры: `counter_id` (string, required)
- API: `GET /management/v1/counter/{counter_id}/segments`

---

### `metrika_list_logs`
Список запросов на выгрузку сырых логов.
- Параметры: `counter_id` (string, required)
- API: `GET /logs/v1/counter/{counter_id}/logrequests`

---

### `metrika_create_log_request`
Создать запрос на выгрузку логов.

| Параметр | Тип | Описание |
|----------|-----|----------|
| `counter_id` | string | ID счётчика |
| `fields` | string | Поля через запятую: `ym:s:visitID,ym:s:date` |
| `source` | string | `visits` или `hits` |
| `date1` | string | Начало: `YYYY-MM-DD` |
| `date2` | string | Конец: `YYYY-MM-DD` |

- API: `POST /logs/v1/counter/{counter_id}/logrequests?fields=...&source=...&date1=...&date2=...`

---

### `metrika_download_log`
Скачать часть выгруженных логов (возвращает TSV).
- Параметры: `counter_id` (string, required), `request_id` (string, required), `part_number` (string, default: `0`)
- API: `GET /logs/v1/counter/{counter_id}/logrequest/{request_id}/part/{part}/download`

## Yandex Metrika API

```
Base URL:         https://api-metrika.yandex.net
Auth header:      Authorization: OAuth <ACCESS_TOKEN>
Accept header:    application/json
HTTP timeout:     30 секунд

Management API:   /management/v1/
Reports API:      /stat/v1/data
Logs API:         /logs/v1/
```

HTTP ошибки возвращаются как `result.isError: true` с текстом `HTTP {код} from {path}: {тело}`.
Тело ответа обрезается до 300 символов в сообщении об ошибке.

Ответы Metrika API возвращаются клиенту как **prettified JSON** (`json.MarshalIndent`).
Для Logs API (TSV формат) — возвращается как есть, без prettify.

## Архитектура

```
cmd/server/main.go
  ├── config.Load()              → читает ACCESS_TOKEN, LOG_LEVEL, LOG_FILE
  ├── metrika.NewClient(token)   → HTTP клиент (30s timeout, WithBaseURL для тестов)
  ├── mcp.NewStdioTransport(     → bufio.Scanner stdin (4MB), json.Encoder stdout
  │       os.Stdin, os.Stdout)
  └── mcp.NewServer(transport, client, logger, version)
          └── .Run(ctx)          → основной цикл readline → dispatch → write

mcp.Server
  ├── transport *StdioTransport
  ├── metrika   *metrika.Client
  ├── logger    *slog.Logger
  ├── version   string
  ├── initialized bool           → true после первого initialize
  └── tools     []Tool           → статический реестр (buildToolRegistry)

StdioTransport
  ├── scanner  *bufio.Scanner    → читает stdin построчно
  ├── encoder  *json.Encoder     → пишет в stdout
  └── mu       sync.Mutex        → защита encoder от concurrent writes

metrika.Client
  ├── baseURL     string
  ├── token       string
  ├── httpClient  *http.Client
  └── get/getRaw/post            → внутренние HTTP методы
```

## Thread Safety

`StdioTransport.WriteResponse` защищён `sync.Mutex` — безопасен для конкурентных горутин:

```go
func (t *StdioTransport) WriteResponse(resp Response) error {
    t.mu.Lock()
    defer t.mu.Unlock()
    return t.encoder.Encode(resp) // Encode атомарно + добавляет \n
}
```

В текущей реализации сервер обрабатывает запросы последовательно в одной горутине — mutex нужен на случай будущих изменений.

## Тестирование

### Unit тесты

```go
// server_test.go — создаём Server с nil metrika client (tools не вызываем)
func newTestServer(t *testing.T) *mockServer {
    var out bytes.Buffer
    transport := NewStdioTransport(strings.NewReader(""), &out)
    logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
    srv := &Server{transport: transport, metrika: nil, logger: logger, version: "test"}
    srv.tools = srv.buildToolRegistry()
    return &mockServer{Server: srv, out: &out}
}

// transport_test.go — тестируем через strings.Reader / bytes.Buffer
// metrika/client_test.go — тестируем с httptest.NewServer
```

### E2E тест через stdin pipe

```bash
export $(grep -v '^#' .env | xargs)   # загрузить ACCESS_TOKEN из .env

{
  printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}\n'
  sleep 0.1
  printf '{"jsonrpc":"2.0","method":"notifications/initialized"}\n'
  sleep 0.1
  printf '{"jsonrpc":"2.0","id":2,"method":"tools/list"}\n'
  sleep 0.2
  printf '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"metrika_get_counters","arguments":{}}}\n'
  sleep 3
} | ACCESS_TOKEN="$ACCESS_TOKEN" ./bin/mcp-server 2>/dev/null
```

> Обрати внимание на `ACCESS_TOKEN="$ACCESS_TOKEN"` — явная передача, не полагаемся на godotenv при pipe.

### MCP Inspector (веб-UI)

```bash
# Устанавливать не нужно — npx скачает автоматически
ACCESS_TOKEN="$ACCESS_TOKEN" npx @modelcontextprotocol/inspector ./bin/mcp-server
# Открывает http://localhost:5173
```

## Конфиг opencode

```jsonc
// opencode.jsonc в корне проекта ИЛИ ~/.config/opencode/opencode.jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "yandex-metrika": {
      "type": "local",
      "command": ["/absolute/path/to/yametrika-mcp"],  // production
      // "command": ["./bin/mcp-server"],               // разработка
      "enabled": true,
      "env": {
        "ACCESS_TOKEN": "y0_..."
      }
    }
  }
}
```

> `command` требует **абсолютный путь** для production. Относительный путь `./bin/mcp-server` работает только если opencode запускается из корня проекта.

## Добавление нового Tool — пошагово

Каждый новый tool затрагивает ровно 4 места:

```
1. internal/<api>/          → добавить метод клиента
2. internal/mcp/server.go   → зарегистрировать в buildToolRegistry()
3. internal/mcp/handlers.go → добавить case в executeTool() + метод toolXxx()
4. internal/<api>/*_test.go → написать тест
```

### Пример: добавить `metrika_get_sources`

**1. API метод** (`internal/metrika/sources.go`):
```go
type Source struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

func (c *Client) GetSources(ctx context.Context, counterID string) ([]Source, error) {
    var resp struct{ Sources []Source `json:"sources"` }
    err := c.get(ctx, "/management/v1/counter/"+counterID+"/sources", nil, &resp)
    if err != nil {
        return nil, fmt.Errorf("GetSources %s: %w", counterID, err)
    }
    return resp.Sources, nil
}
```

**2. Регистрация** в `buildToolRegistry()` (`internal/mcp/server.go`):
```go
{
    Name:        "metrika_get_sources",
    Description: "Получить список источников трафика счётчика",
    InputSchema: InputSchema{
        Type: "object",
        Properties: map[string]Property{
            "counter_id": {Type: "string", Description: "ID счётчика"},
        },
        Required: []string{"counter_id"},
    },
},
```

**3. Handler** (`internal/mcp/handlers.go`):
```go
// В executeTool():
case "metrika_get_sources":
    return s.toolGetSources(ctx, args)

// Новый метод:
func (s *Server) toolGetSources(ctx context.Context, args map[string]any) ToolCallResult {
    id := getString(args, "counter_id")
    if id == "" {
        return errorContent("параметр counter_id обязателен")
    }
    sources, err := s.metrika.GetSources(ctx, id)
    if err != nil {
        return errorContent(fmt.Sprintf("Ошибка получения источников %s: %s", id, err))
    }
    result, _ := jsonText(sources)
    return result
}
```

**4. Тест** (`internal/metrika/sources_test.go`):
```go
func TestGetSources(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path != "/management/v1/counter/123/sources" {
            t.Errorf("unexpected path: %s", r.URL.Path)
        }
        json.NewEncoder(w).Encode(struct{ Sources []Source `json:"sources"` }{
            Sources: []Source{{ID: 1, Name: "Direct"}},
        })
    }))
    defer ts.Close()

    client := NewClient("tok", WithBaseURL(ts.URL))
    sources, err := client.GetSources(context.Background(), "123")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(sources) != 1 {
        t.Errorf("want 1 source, got %d", len(sources))
    }
}
```

## Дизайн Tools для LLM

### Именование

```
<api_prefix>_<action>_<resource>
```

- `metrika_get_counters` ✅ — понятно что делает
- `getData` ❌ — непонятно что за данные
- `metrika_counters` ❌ — непонятно это get или create

Используй глаголы: `get`, `list`, `create`, `update`, `delete`, `download`, `search`.

### Description — главный элемент для LLM

Description должен отвечать на вопросы:
1. **Что делает** tool?
2. **Когда использовать** (в каком контексте LLM должен его выбрать)?
3. **Что возвращает** (структура ответа)?

```go
// ❌ Плохо — слишком кратко
Description: "Получить данные"

// ✅ Хорошо — LLM понимает когда и зачем
Description: "Получить статистический отчёт по метрикам и измерениям. " +
    "Используй для анализа трафика, конверсий, источников за период. " +
    "Возвращает данные с группировкой по указанным dimensions."
```

### InputSchema — помогай LLM заполнять параметры

```go
// ❌ Плохо — LLM не знает формат
"date1": {Type: "string", Description: "Дата начала"}

// ✅ Хорошо — явный формат + примеры
"date1": {
    Type:        "string",
    Description: "Начало периода: YYYY-MM-DD или '7daysAgo', '30daysAgo', 'today', 'yesterday'",
    Default:     "7daysAgo",
}

// ✅ Enum ограничивает выбор
"source": {
    Type:        "string",
    Description: "Тип данных для выгрузки",
    Enum:        []string{"visits", "hits"},
}
```

### Форматирование ответов

```go
// ✅ Prettified JSON — LLM хорошо читает структурированные данные
func jsonText(v any) (ToolCallResult, error) {
    b, err := json.MarshalIndent(v, "", "  ")
    ...
    return textContent(string(b)), nil
}

// ✅ TSV/CSV — возвращай как есть (для логов, экспортов)
return textContent(tsvData)

// ❌ Не возвращай минифицированный JSON — LLM хуже с ним работает
json.Marshal(v)  // без indent
```

### Разбивка на несколько tools vs один универсальный

```
// ✅ Лучше — несколько конкретных tools
metrika_get_counters   — список счётчиков
metrika_get_counter    — один счётчик
metrika_get_report     — отчёт

// ❌ Хуже — один god-tool с кучей опциональных параметров
metrika_get_data(type: "counter"|"report"|"goals", ...)
```

LLM лучше выбирает из набора специализированных tools, чем конфигурирует один большой.
