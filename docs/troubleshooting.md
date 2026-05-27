---
layout: default
title: Решение проблем
nav_order: 11
permalink: /troubleshooting
---

# Решение проблем
{: .no_toc }

<details open markdown="block">
  <summary>Содержание</summary>
  {: .text-delta }
1. TOC
{:toc}
</details>

---

## Сервер не запускается

### `ACCESS_TOKEN is required`

```
fatal: config: ACCESS_TOKEN is required
```

**Причина:** `.env` файл не найден или ключ написан неправильно.

```bash
# Проверить
cat .env

# Неправильно — godotenv загружает ключ как написан
AccessToken=y0_...

# Правильно — код читает os.Getenv("ACCESS_TOKEN")
ACCESS_TOKEN=y0_...
```

**Решение:** Переименуйте ключ в `.env` в `UPPER_SNAKE_CASE`.

---

### `open log file: ...`

Указан `LOG_FILE` путь, но директория не существует.

```bash
# Создать директорию
mkdir -p /var/log/my-app/

# Или убрать LOG_FILE из .env
```

---

## Протокол сломан (клиент не получает ответы)

### Посторонний вывод в stdout

**Симптомы:** opencode видит сервер, но tools не работают или отвечают мусором.

**Причина:** Что-то пишет в `os.Stdout` кроме JSON-RPC encoder.

**Диагностика:**
```bash
# Запустить E2E тест и посмотреть сырой вывод
{
  printf '{"jsonrpc":"2.0","id":1,"method":"initialize",...}\n'
  sleep 0.3
} | ./bin/mcp-server 2>/dev/null | cat -A
```

Если между JSON строками есть что-то, что не начинается с `{` — это проблема.

**Типичные источники:**
```go
// ❌ fmt.Println идёт в stdout
fmt.Println("debug:", something)

// ❌ log.Println без настройки тоже в stdout
log.Println("started")

// ❌ fmt.Printf без \n — сломает следующее сообщение
fmt.Printf("value: %v", x)
```

**Решение:**
```go
// ✅ Только slog в stderr
logger.Debug("value", "x", x)

// ✅ Явно в stderr если нужно
fmt.Fprintln(os.Stderr, "debug:", something)
```

---

## CI fails: gofmt lint error

```
internal/myapi/users.go:11: File is not `gofmt`-ed with `-s`
```

**Причина:** CI использует `golangci-lint` с линтером `gofmt -s`. Простой `gofmt` без флага `-s` недостаточен.

**Решение:**
```bash
# Форматировать с флагом -s (simplify)
gofmt -s -w .

# Проверить что нет нарушений (пустой вывод = OK)
gofmt -s -l .

# НЕ используйте просто gofmt -w . — CI его не примет
```

**Частая причина:** ручное выравнивание полей структур пробелами:
```go
// ❌ Ломает gofmt -s
type User struct {
    ID       int    `json:"id"`
    Name     string `json:"name"`
    LongName string `json:"long_name"` // дополнительные пробелы ломают simplify
}

// ✅ Пусть gofmt сам выровняет
type User struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	LongName string `json:"long_name"`
}
```

Правило: **никогда не добавляйте ручное выравнивание пробелами**, запускайте `gofmt -s -w .` и доверяйте результату.

---

## goreleaser check fails

### `format is deprecated, use formats`

```
   ✗ archives: "format" is deprecated, use "formats" instead
```

**Причина:** goreleaser v2 переименовал поле.

```yaml
# ❌ Старый синтаксис (v1)
archives:
  - format: tar.gz
    format_overrides:
      - goos: windows
        format: zip

# ✅ Новый синтаксис (v2)
archives:
  - formats: [tar.gz]
    format_overrides:
      - goos: windows
        formats: [zip]
```

**Проверка конфига:**
```bash
goreleaser check   # должен вернуть "config is valid"
```

---

### ldflags переменные не инжектируются

**Симптомы:** `version` показывает "dev" даже в релизе.

**Причина:** goreleaser инжектирует `-X main.commit=...` но переменная не объявлена в `main.go`.

```go
// ❌ Go молча игнорирует -X для несуществующих переменных
var version = "dev"

// ✅ Все три должны быть объявлены
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

Классический пример: `snapshot` job с `if: github.event_name == 'pull_request'` в workflow с `on: push: tags:` — такой workflow никогда не запускается при PR.

**Решение:** Проверьте что `if:` совместим с `on:` триггерами workflow. Если job не нужен — уберите его.

---

### Node.js 20 deprecation warning

```
Node.js 20 actions are deprecated. Please update the following actions...
```

**Решение:** Добавьте на уровне workflow:
```yaml
env:
  FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: true
```

---

### goreleaser-check job был skipped

Если добавили условие `if: github.event_name == 'pull_request'` но делаете push прямо в main (без PR) — job пропускается.

**Решение:** Уберите условие для `goreleaser-check`, пусть запускается всегда.

---

## bufio.Scanner: token too long

```
bufio.Scanner: token too long
```

**Причина:** Ответ API превысил буфер scanner (по умолчанию 64 KB).

**Решение:** Увеличьте буфер в `transport.go`:
```go
const maxTokenSize = 4 * 1024 * 1024 // 4 MB
scanner.Buffer(make([]byte, maxTokenSize), maxTokenSize)
```

---

## opencode не видит MCP сервер

### Проверить конфиг

```jsonc
// opencode.jsonc
{
  "mcp": {
    "my-api": {
      "type": "local",
      "command": ["/ABSOLUTE/PATH/to/bin/mcp-server"],  // ← абсолютный путь!
      "enabled": true
    }
  }
}
```

### Проверить что сервер запускается

```bash
# Запустить вручную с тестовым вводом
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}' \
  | ACCESS_TOKEN="$ACCESS_TOKEN" /absolute/path/to/bin/mcp-server
```

Должен вернуть JSON с `"result": {"protocolVersion": "2024-11-05", ...}`.

### Проверить логи

```bash
# Включить DEBUG логи и файл
LOG_LEVEL=debug LOG_FILE=/tmp/mcp-debug.log ./bin/mcp-server
tail -f /tmp/mcp-debug.log
```

---

## go: command not found

```bash
# Go не в PATH — добавить явно
export PATH="$HOME/.local/go/bin:$PATH"

# Или если установлен системно
export PATH="/usr/local/go/bin:$PATH"

# Постоянно — добавить в .bashrc
echo 'export PATH="$HOME/.local/go/bin:$PATH"' >> ~/.bashrc
```

---

## Чеклист "готово к production"

- [ ] `go build ./...` — компилируется без ошибок
- [ ] `gofmt -s -l .` — пустой вывод (нет нарушений)
- [ ] `go vet ./...` — без ошибок
- [ ] `go test ./... -race` — все тесты зелёные
- [ ] `goreleaser check` — конфиг валиден
- [ ] E2E: `initialize` → `tools/list` → `tools/call` работает
- [ ] Логи идут только в stderr (не в stdout)
- [ ] `.env` в `.gitignore`, токены не закоммичены
- [ ] CI зелёный на GitHub
- [ ] opencode видит сервер в `/mcp`
