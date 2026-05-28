# Yandex Metrika MCP Server

[![CI](https://github.com/ivanshamaev/mcp-server/actions/workflows/ci.yml/badge.svg)](https://github.com/ivanshamaev/mcp-server/actions/workflows/ci.yml)
[![Release](https://github.com/ivanshamaev/mcp-server/actions/workflows/release.yml/badge.svg)](https://github.com/ivanshamaev/mcp-server/releases/latest)
[![Go Report Card](https://goreportcard.com/badge/github.com/ivshamaev/yametrika-mcp)](https://goreportcard.com/report/github.com/ivshamaev/yametrika-mcp)

MCP (Model Context Protocol) сервер для интеграции **Yandex Metrika** с opencode, Claude Code и другими MCP-совместимыми клиентами.

Транспорт: **stdio JSON-RPC 2.0** · Протокол: `2025-11-25` · Язык: **Go 1.22**

## Установка

### Скачать готовый бинарник

```bash
# Последняя версия (Linux amd64)
curl -sL https://github.com/ivanshamaev/mcp-server/releases/latest/download/yametrika-mcp_linux_amd64.tar.gz \
  | tar xz && chmod +x yametrika-mcp
```

Доступные платформы: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`

### Собрать из исходников

```bash
git clone https://github.com/ivanshamaev/mcp-server.git
cd mcp-server
make build          # → ./bin/yametrika-mcp (или make build-linux/darwin/windows)
```

## Настройка

```bash
# .env или переменная окружения
ACCESS_TOKEN=y0_...   # OAuth токен Yandex Metrika (обязательно)
LOG_LEVEL=info        # debug | info | error  (опционально)
LOG_FILE=             # путь к файлу лога, по умолчанию stderr (опционально)
```

Получить токен: [OAuth Яндекс](https://oauth.yandex.ru/) → создать приложение с правом `metrika:read`.

## Использование с opencode

```jsonc
// opencode.jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "yandex-metrika": {
      "type": "local",
      "command": ["/absolute/path/to/yametrika-mcp"],
      "enabled": true,
      "env": {
        "ACCESS_TOKEN": "y0_ваш_токен"
      }
    }
  }
}
```

## Доступные инструменты (MCP Tools)

| Tool | Описание |
|------|----------|
| `metrika_get_counters` | Список всех счётчиков |
| `metrika_get_counter` | Данные конкретного счётчика |
| `metrika_get_report` | Статистический отчёт (метрики + измерения) |
| `metrika_get_goals` | Цели счётчика |
| `metrika_get_segments` | Сегменты счётчика |
| `metrika_list_logs` | Список запросов на выгрузку логов |
| `metrika_get_log_request` | Статус конкретного запроса логов |
| `metrika_create_log_request` | Создать выгрузку сырых логов |
| `metrika_download_log` | Скачать часть логов |
| `metrika_clean_log_request` | Удалить запрос логов после скачивания |

## Релизы

Релиз создаётся тегом — GitHub Actions автоматически собирает бинарники для всех платформ и публикует GitHub Release.

```bash
# Посмотреть текущую версию
make version

# Создать тег и запустить релиз (интерактивно: покажет версию и попросит подтверждение)
make tag-patch    # v1.0.0 → v1.0.1  (bugfix)
make tag-minor    # v1.0.1 → v1.1.0  (новые tools, обратно совместимо)
make tag-major    # v1.1.0 → v2.0.0  (breaking changes)
```

Вручную:

```bash
git tag -a v1.2.3 -m "Release v1.2.3"
git push origin v1.2.3
```

Следить за сборкой: `gh run watch` или вкладка Actions на GitHub.

## Разработка

```bash
make test         # юнит тесты
make test-mcp     # E2E тест MCP протокола
make inspect      # MCP Inspector (веб-UI отладки)
make lint         # статический анализ
```

Читай [CLAUDE.md](CLAUDE.md) для разработки с Claude Code.

## Лицензия

MIT
