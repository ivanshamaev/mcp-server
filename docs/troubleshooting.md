# Решение проблем

## Сервер не запускается

### `ACCESS_TOKEN is required`

```
fatal: config: ACCESS_TOKEN is required
```

!!! danger "Причина: неправильный ключ в `.env`"
    `godotenv.Load()` загружает ключ точно так, как записан в файле.
    
    ```bash
    # ❌ Неправильно
    AccessToken=y0_...
    
    # ✅ Правильно (код читает os.Getenv("ACCESS_TOKEN"))
    ACCESS_TOKEN=y0_...
    ```

---

## Протокол сломан

### Посторонний вывод в stdout

**Симптомы:** opencode видит сервер, но tools не работают.

**Диагностика:**
```bash
{
  printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{...}}\n'
  sleep 0.3
} | ./bin/mcp-server 2>/dev/null | cat -A
```

Если между JSON строками есть что-то не начинающееся с `{` — это проблема.

**Типичные источники:**
```go
// ❌ Всё это идёт в stdout
fmt.Println("debug:", something)
log.Println("started")       // log по умолчанию → stdout

// ✅ Только в stderr
logger.Debug("value", "x", x)
fmt.Fprintln(os.Stderr, "debug:", something)
```

---

## CI: gofmt lint error

```
internal/myapi/users.go:11: File is not `gofmt`-ed with `-s`
```

!!! warning "Не просто `gofmt` — нужен флаг `-s`"
    CI использует `golangci-lint` с линтером `gofmt -s`.
    
    ```bash
    # ✅ Правильно
    gofmt -s -w .
    
    # Проверить (пустой вывод = OK)
    gofmt -s -l .
    
    # ❌ Этого недостаточно
    gofmt -w .
    ```

**Частая причина:** ручное выравнивание пробелами в структурах:
```go
// ❌ Ломает gofmt -s
type User struct {
    ID       int    `json:"id"`
    Name     string `json:"name"`
    LongName string `json:"long_name"` // разные отступы
}
```

Правило: **не добавляйте ручное выравнивание пробелами**, запускайте `gofmt -s -w .`.

---

## goreleaser check fails

### `format is deprecated, use formats`

```yaml
# ❌ Старый синтаксис (v1)
archives:
  - format: tar.gz
    format_overrides:
      - { goos: windows, format: zip }

# ✅ Новый синтаксис (v2)
archives:
  - formats: [tar.gz]
    format_overrides:
      - { goos: windows, formats: [zip] }
```

Проверка: `goreleaser check`

### ldflags переменные не инжектируются

```go
// ❌ Go молча игнорирует -X для несуществующих переменных
var version = "dev"

// ✅ Все три должны быть объявлены в main.go
var (
    version = "dev"
    commit  = "none"
    date    = "unknown"
)
```

---

## GitHub Actions

### Jobs показываются как "skipped"

**Причина:** Условие `if:` несовместимо с триггером workflow.

```yaml
# ❌ Мёртвый код: workflow запускается только по тегу, но условие проверяет PR
on:
  push:
    tags: ["v*.*.*"]
jobs:
  snapshot:
    if: github.event_name == 'pull_request'  # никогда не true!
```

**Решение:** Убедитесь что `if:` совместим с `on:` триггерами.

### Node.js 20 deprecation warning

```yaml
# Добавить на уровне workflow
env:
  FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: true
```

---

## `bufio.Scanner: token too long`

Ответ API превысил буфер scanner (по умолчанию 64 KB).

```go
// В transport.go увеличьте буфер:
const maxTokenSize = 4 * 1024 * 1024 // 4 MB
scanner.Buffer(make([]byte, maxTokenSize), maxTokenSize)
```

---

## opencode не видит сервер

=== "Проверить конфиг"

    ```jsonc
    {
      "mcp": {
        "my-api": {
          "type": "local",
          "command": ["/ABSOLUTE/PATH/to/bin/mcp-server"],
          "enabled": true
        }
      }
    }
    ```
    
    Убедитесь что путь **абсолютный**.

=== "Ручной тест"

    ```bash
    echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}' \
      | ACCESS_TOKEN="$ACCESS_TOKEN" /absolute/path/to/bin/mcp-server
    ```
    
    Должен вернуть JSON с `"result": {"protocolVersion": "2024-11-05", ...}`.

=== "Включить debug логи"

    ```bash
    LOG_LEVEL=debug LOG_FILE=/tmp/mcp-debug.log ./bin/mcp-server
    tail -f /tmp/mcp-debug.log
    ```

---

## Чеклист "готово к production"

- [x] `go build ./...` — компилируется без ошибок
- [x] `gofmt -s -l .` — пустой вывод
- [x] `go vet ./...` — без ошибок
- [x] `go test ./... -race` — все тесты зелёные
- [x] `goreleaser check` — конфиг валиден
- [x] E2E: `initialize` → `tools/list` → `tools/call` работает
- [x] Логи только в stderr (не в stdout)
- [x] `.env` в `.gitignore`, токены не закоммичены
- [x] CI зелёный на GitHub
- [x] opencode видит сервер и может вызывать tools
