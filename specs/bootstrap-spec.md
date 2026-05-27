# Bootstrap Spec — MCP Server с нуля

Пошаговое руководство: от пустой директории до работающего MCP сервера для любого API.

## Шаг 0. Предварительные требования

### Go

```bash
# Проверить установку
go version   # нужно >= 1.22

# Если Go не установлен (без sudo):
cd /tmp
curl -sL https://go.dev/dl/go1.22.4.linux-amd64.tar.gz -o go.tar.gz
mkdir -p ~/.local/go
tar -C ~/.local -xzf go.tar.gz
echo 'export PATH="$HOME/.local/go/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
go version
```

### Инструменты разработки

```bash
# После установки Go:
go install github.com/goreleaser/goreleaser/v2@latest
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

## Шаг 1. Создание проекта

```bash
# Создать директорию
mkdir my-api-mcp && cd my-api-mcp
git init

# Инициализировать Go модуль
# Формат: github.com/<user>/<repo>
go mod init github.com/username/my-api-mcp

# Создать структуру директорий
mkdir -p cmd/server internal/{mcp,myapi,config} .claude/commands .github/workflows bin specs
```

## Шаг 2. Скопировать базовые файлы MCP

Следующие файлы **не зависят от конкретного API** — копируются как есть:

```
internal/mcp/types.go      — JSON-RPC типы, MCP типы, helpers
internal/mcp/transport.go  — StdioTransport (bufio.Scanner + json.Encoder)
internal/mcp/server.go     — Server struct, Run(), handleRequest()
internal/config/config.go  — Config struct, Load() с godotenv
cmd/server/main.go         — точка входа
```

Что нужно **адаптировать** под свой API:
- `internal/mcp/server.go` — метод `buildToolRegistry()` — список tools
- `internal/mcp/handlers.go` — `executeTool()` — диспетчер вызовов
- `internal/myapi/` — HTTP клиент к своему API

## Шаг 3. Добавить зависимости

```bash
# Обязательно
go get github.com/joho/godotenv

# Синхронизировать go.mod и go.sum
go mod tidy

# go.sum ВСЕГДА коммитится вместе с go.mod!
```

## Шаг 4. Настроить .env

```bash
# .env (секреты, в .gitignore)
API_TOKEN=your_token_here
API_BASE_URL=https://api.example.com   # опционально, для переопределения

# .env.example (шаблон, коммитится)
API_TOKEN=
# API_BASE_URL=https://api.example.com
```

**Правило именования:** всегда `UPPER_SNAKE_CASE`. `godotenv.Load()` загружает ключ ровно так, как он написан в файле. Если код читает `os.Getenv("API_TOKEN")`, в `.env` должно быть `API_TOKEN=`.

## Шаг 5. Создать API клиент

Минимальный шаблон клиента для любого REST API:

```go
// internal/myapi/client.go
package myapi

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "time"
)

const defaultBaseURL = "https://api.example.com"

type Client struct {
    baseURL    string
    token      string
    httpClient *http.Client
}

type Option func(*Client)

func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = u } }
func WithTimeout(d time.Duration) Option { return func(c *Client) { c.httpClient.Timeout = d } }

func NewClient(token string, opts ...Option) *Client {
    c := &Client{
        baseURL: defaultBaseURL,
        token:   token,
        httpClient: &http.Client{Timeout: 30 * time.Second},
    }
    for _, o := range opts {
        o(c)
    }
    return c
}

// get — универсальный GET с JSON декодированием
func (c *Client) get(ctx context.Context, path string, query url.Values, dst any) error {
    u := c.baseURL + path
    if len(query) > 0 {
        u += "?" + query.Encode()
    }
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
    if err != nil {
        return fmt.Errorf("build request: %w", err)
    }
    // Адаптируй заголовок под свой API:
    req.Header.Set("Authorization", "Bearer "+c.token)  // Bearer
    // req.Header.Set("X-API-Key", c.token)              // API Key
    req.Header.Set("Accept", "application/json")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("http get %s: %w", path, err)
    }
    defer resp.Body.Close()
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return fmt.Errorf("read body: %w", err)
    }
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("HTTP %d from %s: %.300s", resp.StatusCode, path, body)
    }
    return json.Unmarshal(body, dst)
}
```

### Паттерны аутентификации

| API тип | Заголовок |
|---------|-----------|
| Bearer / OAuth | `Authorization: Bearer <token>` |
| API Key (header) | `X-API-Key: <key>` или `Authorization: ApiKey <key>` |
| Basic Auth | `Authorization: Basic <base64(user:pass)>` |
| OAuth Yandex | `Authorization: OAuth <token>` |
| Query param | `?api_key=<key>` (добавить в `url.Values`) |

## Шаг 6. Добавить первый Tool

Каждый новый tool — 4 файла:

### 6.1 API метод в `internal/myapi/`

```go
// internal/myapi/items.go
type Item struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

type itemsResponse struct {
    Items []Item `json:"items"`
}

func (c *Client) GetItems(ctx context.Context) ([]Item, error) {
    var resp itemsResponse
    if err := c.get(ctx, "/v1/items", nil, &resp); err != nil {
        return nil, fmt.Errorf("GetItems: %w", err)
    }
    return resp.Items, nil
}
```

### 6.2 Зарегистрировать tool в `internal/mcp/server.go`

В метод `buildToolRegistry()`:

```go
{
    Name:        "myapi_get_items",
    Description: "Получить список элементов. Описание должно помочь LLM понять КОГДА использовать этот tool.",
    InputSchema: InputSchema{
        Type: "object",
        Properties: map[string]Property{
            // параметры tool
        },
        Required: []string{},
    },
},
```

### 6.3 Добавить case в `internal/mcp/handlers.go`

```go
func (s *Server) executeTool(ctx context.Context, name string, args map[string]any) ToolCallResult {
    switch name {
    case "myapi_get_items":
        return s.toolGetItems(ctx)
    // ...
    }
}

func (s *Server) toolGetItems(ctx context.Context) ToolCallResult {
    items, err := s.myapi.GetItems(ctx)
    if err != nil {
        return errorContent("Ошибка получения элементов: " + err.Error())
    }
    result, _ := jsonText(items)
    return result
}
```

### 6.4 Написать тест

```go
// internal/myapi/items_test.go
func TestGetItems(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(itemsResponse{Items: []Item{{ID: 1, Name: "Test"}}})
    }))
    defer ts.Close()

    client := NewClient("tok", WithBaseURL(ts.URL))
    items, err := client.GetItems(context.Background())
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(items) != 1 {
        t.Errorf("want 1 item, got %d", len(items))
    }
}
```

## Шаг 7. Первая сборка и тест

```bash
# Сборка
go build -o bin/mcp-server ./cmd/server

# Unit тесты
go test ./... -v

# E2E тест протокола
{
  printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}\n'
  sleep 0.1
  printf '{"jsonrpc":"2.0","method":"notifications/initialized"}\n'
  sleep 0.1
  printf '{"jsonrpc":"2.0","id":2,"method":"tools/list"}\n'
  sleep 0.3
} | API_TOKEN="$API_TOKEN" ./bin/mcp-server 2>/dev/null
```

Ожидаемый вывод:
```json
{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05",...}}
{"jsonrpc":"2.0","id":2,"result":{"tools":[...]}}
```

## Шаг 8. Настроить opencode

```jsonc
// opencode.jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "my-api": {
      "type": "local",
      "command": ["/absolute/path/to/bin/mcp-server"],
      "enabled": true,
      "env": { "API_TOKEN": "your_token" }
    }
  }
}
```

После сохранения — перезапустить opencode. Проверить что server виден в `/mcp` или аналогичной команде.

## Шаг 9. Git setup

```bash
# .gitignore
cat > .gitignore << 'EOF'
.env
bin/
dist/
coverage.out
coverage.html
go.work
go.work.sum
EOF

# Первый коммит
git add .
git commit -m "feat: initial MCP server implementation

- stdio JSON-RPC 2.0 transport (MCP spec 2024-11-05)
- tools: myapi_get_items
- GitHub Actions CI + goreleaser Release"

# Настроить remote
git remote add origin git@github.com:username/my-api-mcp.git
git branch -M main
git push -u origin main
```

## Шаг 10. GitHub репозиторий

### Обязательно

1. **Settings → Actions → General → Workflow permissions:**
   Выбрать "Read and write permissions" — иначе goreleaser не сможет создавать Releases.

2. **Settings → Branches → Branch protection rules** (для `main`):
   - Require status checks before merging: `test`, `lint`
   - Require branches to be up to date before merging
   - Do not allow bypassing the above settings

### Опционально

3. **Settings → Secrets → Actions**: если API требует секреты недоступные через env (нужен `GITHUB_TOKEN` уже доступен автоматически)

## Чеклист "готово к использованию"

- [ ] `go build ./...` — компилируется без ошибок
- [ ] `gofmt -s -l .` — пустой вывод (нет нарушений)
- [ ] `go vet ./...` — без ошибок
- [ ] `go test ./... -race` — все тесты зелёные
- [ ] `goreleaser check` — конфиг валиден
- [ ] E2E тест: `initialize` → `tools/list` → `tools/call` — ответы корректны
- [ ] opencode видит server и может вызывать tools
- [ ] CI зелёный на GitHub
