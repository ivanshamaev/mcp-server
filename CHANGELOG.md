# Changelog

Все значимые изменения проекта фиксируются здесь.
Формат основан на [Keep a Changelog](https://keepachangelog.com/ru/1.1.0/).
Версионирование: [Semantic Versioning](https://semver.org/lang/ru/).

## Схема версий

```
v MAJOR . MINOR . PATCH
   │        │       └── Bugfix: исправление ошибок, обратно совместимо
   │        └────────── Minor: новый функционал, обратно совместимо
   └─────────────────── Major: breaking changes (новый протокол, переименование tools)
```

Суффиксы pre-release: `-alpha.1`, `-beta.1`, `-rc.1`

## [Unreleased]

### Added
- Начальная реализация MCP Server на Go
- Stdio JSON-RPC 2.0 транспорт (MCP spec `2024-11-05`)
- Интеграция с Yandex Metrika API:
  - `metrika_get_counters` — список счётчиков
  - `metrika_get_counter` — данные счётчика
  - `metrika_get_report` — отчёты (Reports API)
  - `metrika_get_goals` — цели счётчика
  - `metrika_get_segments` — сегменты счётчика
  - `metrika_list_logs` — запросы логов
  - `metrika_create_log_request` — создание выгрузки логов
  - `metrika_download_log` — скачивание части логов
- Конфиг через env vars / `.env` файл (godotenv)
- Structured logging через `log/slog` в stderr
- Graceful shutdown на SIGTERM/SIGINT
- CI GitHub Actions (тесты, сборка, lint)
- Release GitHub Actions (goreleaser, multi-platform)

[Unreleased]: https://github.com/ivanshamaev/mcp-server/compare/HEAD
