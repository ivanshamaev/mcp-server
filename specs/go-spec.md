# Go Development Specification

## Версия и окружение

- Минимальная версия: **Go 1.22**
- Бинарник Go: `~/.local/go/bin/go` (установлен без sudo из go.dev/dl)
- В bash-сессиях без `.bashrc`: `export PATH="/home/ivan/.local/go/bin:$PATH"`
- Используй возможности 1.22+: `range` по целым числам, `slices`/`maps` из stdlib, `log/slog`

## Модуль

```
module github.com/ivshamaev/yametrika-mcp
go 1.22
```

GitHub репозиторий: `https://github.com/ivanshamaev/mcp-server`
> Замечание: module path (`ivshamaev`) и github org (`ivanshamaev`) — разные написания, это исторически сложилось.

## Структура пакетов

```
cmd/server/main.go    — точка входа, только сборка зависимостей и запуск
internal/config/      — загрузка конфигурации из env / .env файла
internal/mcp/         — MCP Server: транспорт, роутер, типы
internal/metrika/     — HTTP клиент Yandex Metrika API
```

**Правила:**
- `cmd/` — только `main.go`, вся бизнес-логика в `internal/`
- `internal/` — не может быть импортирован снаружи модуля
- Никаких `pkg/` — это анти-паттерн для этого проекта
- Каждый пакет — одна ответственность

## Бинарные артефакты

| Контекст | Путь | Имя | Как создаётся |
|----------|------|-----|---------------|
| Локальная разработка | `./bin/mcp-server` | `mcp-server` | `make build` / `go build` |
| GitHub Release | `yametrika-mcp` | `yametrika-mcp` | goreleaser |

> Различие важно: в `opencode.jsonc` для production указывай путь к `yametrika-mcp`, для разработки — к `bin/mcp-server`.

## Build-time переменные (ldflags)

В `cmd/server/main.go` объявлены три переменные, которые goreleaser инжектирует через `-ldflags`:

```go
var (
    version = "dev"     // -X main.version={{ .Version }}
    commit  = "none"    // -X main.commit={{ .ShortCommit }}
    date    = "unknown" // -X main.date={{ .Date }}
)
```

Локальная сборка:
```bash
CGO_ENABLED=0 go build \
  -ldflags="-s -w -X main.version=$(git describe --tags --always --dirty)" \
  -o bin/mcp-server ./cmd/server
```

> **Важно:** если добавляешь новую переменную в ldflags — объявляй её в `main.go`. Go молча игнорирует `-X` для несуществующих переменных.

## Зависимости

Политика: **минимум внешних зависимостей**.

Текущий `go.mod`:
```
require github.com/joho/godotenv v1.5.1
```

Разрешено добавлять:
- `github.com/joho/godotenv` — загрузка `.env` (уже есть)

Запрещено без явного согласования:
- ORM / query builders
- Большие фреймворки (echo, gin, cobra...)
- Дублирование стандартной библиотеки

**Тесты** используют только стандартный `testing` пакет — `testify` в `go.mod` нет и добавлять не нужно. Используй `t.Fatal`, `t.Error`, `t.Errorf`.

MCP протокол реализован **с нуля** — без сторонних MCP SDK.

## Форматирование и линтинг

### gofmt -s (обязательно!)

CI использует `golangci-lint` с `gofmt` линтером, который применяет флаг **`-s` (simplify)**. Простой `gofmt -w .` без `-s` недостаточен.

```bash
# Правильно — то что делает CI
gofmt -s -w .

# Проверить без изменений (пустой вывод = всё чисто)
gofmt -s -l .
```

### Выравнивание полей структур

`gofmt` выравнивает поля структур **табуляцией**, не пробелами. Ручное выравнивание пробелами ломает `gofmt -s`:

```go
// ❌ Ручное выравнивание пробелами — ЛОМАЕТ gofmt -s
type Counter struct {
    ID       int    `json:"id"`
    Status   string `json:"status"`
    OwnerLogin string `json:"owner_login"` // разная длина имён → ошибка lint
}

// ✅ Пусть gofmt сам выровняет через табы
type Counter struct {
    ID         int    `json:"id"`
    Status     string `json:"status"`
    OwnerLogin string `json:"owner_login"`
}
```

Правило: **никогда не добавляй пробелы для визуального выравнивания полей структур вручную**. Запускай `gofmt -s -w .` и доверяй ему.

### Полный набор команд

```bash
gofmt -s -w .              # форматирование (обязательно -s)
go vet ./...               # статический анализ
go mod tidy                # синхронизация go.mod / go.sum
golangci-lint run ./...    # полный линтинг (если установлен)
```

Конфиг линтера: `.golangci.yml` в корне проекта.

## Обработка ошибок

```go
// ✅ Wrap с контекстом
if err != nil {
    return fmt.Errorf("GetCounters: %w", err)
}

// ✅ Sentinel errors
var ErrNotFound = errors.New("not found")

// ❌ Потеря контекста
return err

// ❌ Запрещено в production
panic("something went wrong")
```

**MCP tool errors** возвращаются не через Go `error`, а через `ToolCallResult.IsError = true` с текстом в `content`. JSON-RPC `error` только для протокольных сбоев.

## HTTP Клиент

```go
client := &http.Client{
    Timeout: 30 * time.Second, // всегда явный таймаут
}

// Context для отмены — обязателен
req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
if err != nil {
    return fmt.Errorf("build request: %w", err)
}

// Всегда закрывай body
defer resp.Body.Close()
body, err := io.ReadAll(resp.Body)
```

## Конкурентность

```go
// context.Context — первый аргумент каждой функции с I/O
func (c *Client) GetCounters(ctx context.Context) ([]Counter, error)

// sync.Mutex для защиты общего состояния (пример: StdioTransport.mu)
// errgroup для параллельных задач с общей обработкой ошибок
```

## Логирование

**Критическое правило:** `os.Stdout` зарезервирован для JSON-RPC. Любой вывод в stdout сломает MCP протокол.

```go
// ✅ Логи в stderr
logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))

// ✅ Или в файл (задаётся через LOG_FILE env var)
f, _ := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
logger := slog.New(slog.NewJSONHandler(f, nil))

// Структурированные логи
logger.Info("tool called", "tool", name)
logger.Debug("rpc", "method", method, "id", id)
logger.Error("api error", "err", err, "status", code)
```

Уровни:
- `DEBUG` — входящие/исходящие JSON-RPC сообщения
- `INFO` — жизненный цикл сервера, вызовы tools
- `ERROR` — ошибки API, протокольные сбои

## Конфигурация

Переменные окружения (загружаются через `godotenv.Load()` из `.env`):

| Переменная | Обязательна | По умолчанию | Описание |
|------------|:-----------:|--------------|----------|
| `ACCESS_TOKEN` | ✅ | — | OAuth токен Yandex Metrika |
| `CLIENT_ID` | ❌ | — | OAuth ClientID приложения |
| `CLIENT_SECRET` | ❌ | — | OAuth ClientSecret |
| `METRIKA_BASE_URL` | ❌ | `https://api-metrika.yandex.net` | Для тестов с mock сервером |
| `LOG_LEVEL` | ❌ | `info` | `debug` / `info` / `error` |
| `LOG_FILE` | ❌ | stderr | Путь к файлу лога |

> **Внимание:** `.env` файл должен использовать `ACCESS_TOKEN=` (UPPER_SNAKE_CASE). Исторически `.env` был создан с `AccessToken=` — это ошибка, исправлено на `ACCESS_TOKEN=`.

Паттерн functional options для клиентов:
```go
type Option func(*Client)

func WithBaseURL(url string) Option {
    return func(c *Client) { c.baseURL = url }
}

func WithTimeout(d time.Duration) Option {
    return func(c *Client) { c.httpClient.Timeout = d }
}
```

## Тесты

Тесты используют только стандартный пакет `testing` + `net/http/httptest` для mock-сервера:

```go
func TestGetCounters(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Header.Get("Authorization") != "OAuth test-token" {
            t.Errorf("unexpected auth: %s", r.Header.Get("Authorization"))
        }
        json.NewEncoder(w).Encode(countersResponse{
            Counters: []Counter{{ID: 1, Name: "Test"}},
        })
    }))
    defer ts.Close()

    client := NewClient("test-token", WithBaseURL(ts.URL))
    counters, err := client.GetCounters(context.Background())
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(counters) != 1 {
        t.Errorf("want 1 counter, got %d", len(counters))
    }
}
```

Запуск:
```bash
go test ./... -v -race -timeout 60s
go test ./... -coverprofile=coverage.out && go tool cover -func=coverage.out
```

## Именование

```go
package metrika   // строчные, короткие, без подчёркиваний
package mcp

type Server struct{}     // экспортируемые — PascalCase
type Counter struct{}

type jsonRPCRequest struct{} // неэкспортируемые — camelCase

// Интерфейсы — -er суффикс
type ToolHandler interface {
    Handle(ctx context.Context, args map[string]any) (ToolCallResult, error)
}

// Конструкторы — New + тип
func NewServer(transport *StdioTransport, mc *metrika.Client, ...) *Server
func NewClient(token string, opts ...Option) *Client
```

## Важные инварианты

1. **stdout = только JSON-RPC** — ни одной строки вне протокола
2. **context.Context первым** — в каждой функции с I/O или сетевым вызовом
3. **gofmt -s** — не просто `gofmt`, именно с флагом `-s`
4. **ldflags переменные объявлены** — если добавляешь `-X main.foo=...` в ldflags, объяви `var foo string` в `main.go`
5. **go.sum коммитится** — всегда вместе с `go.mod`
6. **No global state** — конфигурация через конструкторы и DI
