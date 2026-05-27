# Go Development Specification

## Версия Go

Минимальная версия: **Go 1.22**. Используй возможности 1.22+:
- `range` по целым числам
- `slices` и `maps` пакеты из стандартной библиотеки
- Улучшенный `http.ServeMux` с методами и wildcards

## Модуль

```
module github.com/ivshamaev/yametrika-mcp

go 1.22
```

## Структура пакетов

```
cmd/server/     — точки входа (main пакеты), минимум логики
internal/       — весь приватный код проекта
  config/       — конфигурация и .env загрузка
  mcp/          — MCP Server реализация
  metrika/      — Yandex Metrika API клиент
```

**Правила:**
- `cmd/` — только `main.go`, вся логика в `internal/`
- `internal/` — нет публичного импорта снаружи модуля
- Никаких `pkg/` — это анти-паттерн для этого проекта
- Каждый пакет — одна ответственность

## Обработка ошибок

```go
// ✅ Правильно — wrap с контекстом
if err != nil {
    return fmt.Errorf("get counter %d: %w", counterID, err)
}

// ✅ Sentinel errors для известных случаев
var ErrNotFound = errors.New("not found")
var ErrUnauthorized = errors.New("unauthorized")

// ❌ Неправильно — теряем контекст
return err

// ❌ Запрещено в production коде
panic("something went wrong")
```

## Зависимости

Политика: **минимум внешних зависимостей**.

Разрешено:
- `github.com/joho/godotenv` — загрузка `.env` файлов
- `github.com/stretchr/testify` — тесты (только в test зависимостях)

Запрещено без явного согласования:
- ORM / query builders
- Большие фреймворки
- Дублирование стандартной библиотеки

MCP протокол реализуем **с нуля** — без сторонних MCP SDK.

## HTTP Клиент

```go
// Всегда явный таймаут
client := &http.Client{
    Timeout: 30 * time.Second,
}

// Используй context для отмены
req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)

// Всегда читай и закрывай тело ответа
defer resp.Body.Close()
body, err := io.ReadAll(resp.Body)
```

## Конкурентность

```go
// sync.Mutex для защиты общего состояния
// errgroup для параллельных задач с обработкой ошибок
// context.Context первым аргументом всегда

func (c *Client) GetCounters(ctx context.Context) ([]Counter, error) { ... }
```

## Логирование

- Все логи → `os.Stderr` (stdout зарезервирован для MCP JSON-RPC)
- Используй `log/slog` (Go 1.21+)
- Уровни: DEBUG для входящих/исходящих JSON-RPC сообщений, INFO для событий, ERROR для ошибок

```go
logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))

// Структурированные логи
logger.Info("tool called", "tool", name, "args", args)
logger.Debug("rpc request", "method", method, "id", id)
logger.Error("api error", "err", err, "status", statusCode)
```

## Тесты

```
internal/mcp/server_test.go      — unit тесты MCP Server
internal/mcp/transport_test.go   — тесты транспорта
internal/metrika/client_test.go  — тесты с mock HTTP сервером
```

```go
// Mock HTTP для Yandex Metrika API
func TestGetCounters(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        assert.Equal(t, "OAuth test_token", r.Header.Get("Authorization"))
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(CountersResponse{Counters: []Counter{{ID: 1, Name: "Test"}}})
    }))
    defer ts.Close()

    client := NewClient("test_token", WithBaseURL(ts.URL))
    counters, err := client.GetCounters(context.Background())
    require.NoError(t, err)
    assert.Len(t, counters, 1)
}
```

## Форматирование и Линтинг

```bash
# Обязательно перед коммитом
gofmt -w .
go vet ./...
golangci-lint run ./...
```

`.golangci.yml` конфиг:
```yaml
linters:
  enable:
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gocritic
    - gofmt
    - goimports
```

## Именование

```go
// Пакеты — строчные, короткие, без подчёркиваний
package metrika
package mcp
package config

// Экспортируемые типы — PascalCase
type Server struct {}
type Counter struct {}

// Неэкспортируемые — camelCase
type jsonRPCRequest struct {}

// Интерфейсы — глагол или -er суффикс
type Handler interface { Handle(ctx context.Context, req Request) (Response, error) }
type ToolExecutor interface { Execute(ctx context.Context, args map[string]any) (any, error) }

// Конструкторы — New + тип
func NewServer(cfg Config) *Server {}
func NewClient(token string, opts ...Option) *Client {}
```

## Конфигурация

```go
// Паттерн functional options для конфигурации
type Option func(*Client)

func WithBaseURL(url string) Option {
    return func(c *Client) { c.baseURL = url }
}

func WithTimeout(d time.Duration) Option {
    return func(c *Client) { c.httpClient.Timeout = d }
}
```

## Сборка

```bash
# Локальная сборка
go build -o bin/mcp-server ./cmd/server

# Production сборка (статическая, без CGO)
CGO_ENABLED=0 go build \
  -ldflags="-s -w -X main.version=$(git describe --tags --always)" \
  -o bin/mcp-server \
  ./cmd/server

# Кросс-компиляция
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bin/mcp-server-linux ./cmd/server
```

## Важные правила

1. **stdout только для JSON-RPC** — любой другой вывод в stdout сломает MCP протокол
2. **Context propagation** — `context.Context` первый аргумент каждой функции с I/O
3. **Graceful shutdown** — обрабатывай `SIGTERM`/`SIGINT`, дочитывай текущий запрос
4. **No global state** — конфигурация через конструкторы и DI
5. **Идемпотентность** — повторный вызов `initialize` должен работать корректно
