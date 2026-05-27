# Yandex Metrika MCP Server

Go-реализация MCP (Model Context Protocol) сервера для интеграции с Yandex Metrika API.
Работает через **stdio JSON-RPC 2.0** — стандартный транспорт для локальных MCP-серверов.

## Быстрый старт

```bash
# Сборка
make build

# Запуск (в обычном режиме его запускает opencode/Claude Code автоматически)
./bin/mcp-server

# Отладка через MCP Inspector
make inspect

# Тесты
make test

# Тесты MCP протокола вручную (stdin pipe)
make test-mcp
```

## Структура проекта

```
mcp-server/
├── CLAUDE.md                   # Этот файл — старт сессии
├── specs/
│   ├── go-spec.md              # Стандарты Go: форматирование, ошибки, тесты, сборка
│   ├── mcp-server-spec.md      # MCP протокол: типы, методы, tools, архитектура
│   └── cicd-spec.md            # CI/CD: GitHub Actions, goreleaser, версионирование
├── cmd/server/
│   └── main.go                 # Точка входа
├── internal/
│   ├── config/
│   │   └── config.go           # Загрузка .env / env vars
│   ├── mcp/
│   │   ├── server.go           # MCP Server: жизненный цикл
│   │   ├── transport.go        # Stdio JSON-RPC транспорт
│   │   ├── handlers.go         # Обработчики tools/call
│   │   └── types.go            # JSON-RPC и MCP типы
│   └── metrika/
│       ├── client.go           # HTTP клиент Yandex Metrika
│       ├── counters.go         # Management API: счётчики
│       ├── reports.go          # Reports API: отчёты
│       ├── goals.go            # Management API: цели
│       └── logs.go             # Logs API: сырые данные
├── .claude/commands/
│   ├── build.md                # /build — собрать бинарник
│   ├── test-mcp.md             # /test-mcp — тест протокола
│   ├── inspect.md              # /inspect — MCP Inspector
│   ├── lint.md                 # /lint — golangci-lint
│   └── release.md              # /release — выпустить новую версию
├── .github/
│   └── workflows/
│       ├── ci.yml              # CI: тесты + lint на push/PR
│       └── release.yml         # Release: goreleaser на тег v*.*.*
├── .env                        # Секреты (в .gitignore)
├── .env.example                # Пример конфига
├── .goreleaser.yml             # Конфиг горелизера (multi-platform сборка)
├── go.mod
├── Makefile
├── CHANGELOG.md                # История изменений (Keep a Changelog)
├── README.md                   # Публичная документация
└── opencode.jsonc              # Конфиг opencode (MCP клиент)
```

## Переменные окружения

| Переменная     | Обязательная | Описание                          |
|----------------|:------------:|-----------------------------------|
| `ACCESS_TOKEN` | ✅           | OAuth токен Yandex Metrika        |
| `CLIENT_ID`    | ❌           | ClientID OAuth приложения         |
| `CLIENT_SECRET`| ❌           | ClientSecret OAuth приложения     |
| `LOG_LEVEL`    | ❌           | `debug`/`info`/`error` (default: `info`) |
| `LOG_FILE`     | ❌           | Путь к файлу лога (stderr если не задан) |

> **Важно:** MCP сервер общается через stdout/stdin. Все логи — **только в stderr** или файл.
> Любой вывод в stdout, кроме JSON-RPC ответов, сломает протокол.

## MCP Tools (доступные инструменты)

| Tool | Описание |
|------|----------|
| `metrika_get_counters` | Список всех счётчиков |
| `metrika_get_counter` | Информация о конкретном счётчике |
| `metrika_get_report` | Отчёт по метрикам и измерениям |
| `metrika_get_goals` | Список целей счётчика |
| `metrika_get_segments` | Список сегментов |
| `metrika_request_log` | Создать запрос на выгрузку логов |
| `metrika_get_log` | Получить данные лога |
| `metrika_list_logs` | Список запросов логов |

## Интеграция с opencode

```jsonc
// opencode.jsonc (в корне проекта или ~/.config/opencode/opencode.jsonc)
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "yandex-metrika": {
      "type": "local",
      "command": ["/absolute/path/to/bin/mcp-server"],
      "enabled": true,
      "env": {
        "ACCESS_TOKEN": "your_token_here"
      }
    }
  }
}
```

## Yandex Metrika API

- **Base URL:** `https://api-metrika.yandex.net`
- **Management API:** `/management/v1/`
- **Reports API:** `/stat/v1/data`
- **Logs API:** `/logs/v1/`
- **Auth Header:** `Authorization: OAuth <ACCESS_TOKEN>`
- **Документация:** https://yandex.ru/dev/metrika/doc/api2/concept/about.html

## Разработка

### Соглашения

- Читай [specs/go-spec.md](specs/go-spec.md) перед написанием кода
- Читай [specs/mcp-server-spec.md](specs/mcp-server-spec.md) для понимания протокола
- Ошибки — через `fmt.Errorf("контекст: %w", err)`
- Никакого `panic` в production коде
- Тесты рядом с кодом: `foo_test.go` в том же пакете
- Используй `internal/` для всего непубличного

### Добавление нового Tool

1. Добавь обработчик в `internal/metrika/`
2. Зарегистрируй tool в `internal/mcp/server.go` (список `tools`)
3. Добавь case в `internal/mcp/handlers.go`
4. Напиши тест в `internal/mcp/handlers_test.go`

### Отладка MCP протокола

```bash
# MCP Inspector (веб-интерфейс)
make inspect

# Ручной тест через stdin
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | ./bin/mcp-server

# Тест конкретного tool
echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"metrika_get_counters","arguments":{}}}' | ACCESS_TOKEN=<token> ./bin/mcp-server
```

### Кастомные команды Claude Code

| Команда | Описание |
|---------|----------|
| `/build` | Собрать бинарник `./bin/mcp-server` |
| `/test-mcp` | Прогнать MCP протокол тест (init + tools/list + call) |
| `/inspect` | Запустить MCP Inspector |
| `/lint` | Запустить golangci-lint |
| `/release` | Выпустить новую версию (patch/minor/major) |

## GitHub Flow и версионирование

### Схема разработки

```
main ──────────────────────────────────────────► (защищена)
  │              ▲                    ▲
  │   feature/   │   squash merge     │
  └──► add-tool ─┘                    │
  │                                   │
  └──────────────────── v1.2.3 ──────►│ (тег → GitHub Release)
```

**Правила:**
- `main` — всегда рабочий код, CI должен быть зелёный
- Каждая задача — отдельная ветка от `main`
- PR → squash merge в `main`
- Релиз = annotated git tag `vMAJOR.MINOR.PATCH`

### Рабочий процесс

```bash
# 1. Создать ветку для задачи
git checkout -b feature/add-sources-tool

# 2. Разработать → тесты → коммит
git commit -m "feat: add metrika_get_sources tool"

# 3. Push и PR
git push -u origin feature/add-sources-tool
gh pr create --title "feat: metrika_get_sources" --body "..."

# 4. После merge → обновить CHANGELOG.md
# 5. Выпустить версию
make tag-minor   # интерактивный тег + push → CI/CD
```

### Схема версий (Semantic Versioning)

| Тип | Когда | Пример |
|-----|-------|--------|
| `patch` | Bugfix, нет новых features | `v1.0.0 → v1.0.1` |
| `minor` | Новые tools/функции, обратно совместимо | `v1.0.1 → v1.1.0` |
| `major` | Breaking changes (переименование tools, смена протокола) | `v1.1.0 → v2.0.0` |

### Что происходит при теге `v*.*.*`

```
git tag -a v1.1.0 -m "Release v1.1.0"
git push origin v1.1.0
         │
         ▼
GitHub Actions: release.yml
  ├── go test ./... -race
  ├── goreleaser build:
  │   ├── linux/amd64, linux/arm64
  │   ├── darwin/amd64, darwin/arm64
  │   └── windows/amd64
  ├── tar.gz + .zip архивы
  ├── checksums.txt (SHA256)
  └── GitHub Release (авто-changelog из коммитов)
```

### Локальная проверка перед релизом

```bash
make release-dry-run   # goreleaser snapshot + lint (без публикации)
ls dist/               # проверь артефакты
```

### CI статусы

- **CI** (push/PR): тесты на Go 1.22+1.23, lint, сборка 5 платформ
- **Release** (тег): полный goreleaser, загрузка на GitHub Releases
- **Snapshot** (PR): проверка goreleaser без публикации
