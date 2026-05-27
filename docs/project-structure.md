---
layout: default
title: Структура проекта
nav_order: 4
permalink: /project-structure
---

# Структура проекта
{: .no_toc }

<details open markdown="block">
  <summary>Содержание</summary>
  {: .text-delta }
1. TOC
{:toc}
</details>

---

## Создание проекта

```bash
# Создать директорию и инициализировать модуль
mkdir my-api-mcp && cd my-api-mcp
git init
go mod init github.com/username/my-api-mcp

# Добавить единственную внешнюю зависимость
go get github.com/joho/godotenv

# Создать структуру директорий
mkdir -p cmd/server internal/{mcp,myapi,config} \
         .github/workflows bin specs docs
```

---

## Дерево файлов

```
my-api-mcp/
├── cmd/
│   └── server/
│       └── main.go          ← точка входа (только wiring)
│
├── internal/
│   ├── mcp/
│   │   ├── types.go         ← JSON-RPC + MCP типы
│   │   ├── transport.go     ← StdioTransport (stdin/stdout)
│   │   ├── server.go        ← Server + lifecycle + tool registry
│   │   ├── handlers.go      ← executeTool + toolXxx() методы
│   │   ├── transport_test.go
│   │   └── server_test.go
│   │
│   ├── myapi/               ← HTTP клиент к вашему API
│   │   ├── client.go        ← Client, Options, get/post helpers
│   │   ├── items.go         ← GetItems() и связанные типы
│   │   ├── items_test.go
│   │   └── ...
│   │
│   └── config/
│       └── config.go        ← Config struct + Load() из env
│
├── .github/
│   └── workflows/
│       ├── ci.yml           ← test + lint + build + goreleaser-check
│       ├── release.yml      ← горелизер по тегу
│       └── docs.yml         ← GitHub Pages деплой
│
├── docs/                    ← документация (GitHub Pages)
├── specs/                   ← внутренние спецификации проекта
│
├── .env                     ← секреты (в .gitignore!)
├── .env.example             ← шаблон (коммитится)
├── .gitignore
├── .golangci.yml            ← конфиг линтера
├── .goreleaser.yml          ← конфиг сборки/релизов
├── go.mod
├── go.sum                   ← ВСЕГДА коммитится вместе с go.mod
├── Makefile                 ← удобные команды
├── opencode.jsonc           ← конфиг для opencode
└── README.md
```

---

## Правила пакетов

### `cmd/server/main.go` — только wiring

```go
// ✅ Правильно: только сборка зависимостей и запуск
func run() error {
    cfg, _ := config.Load()
    mc := myapi.NewClient(cfg.Token)
    transport := mcp.NewStdioTransport(os.Stdin, os.Stdout)
    server := mcp.NewServer(transport, mc, logger, version)
    return server.Run(ctx)
}

// ❌ Неправильно: бизнес-логика в main.go
```

### `internal/` — защита от внешнего импорта

Пакеты в `internal/` не могут быть импортированы из-за пределов модуля — это защита от случайного использования как библиотеки.

### `internal/mcp/` — не трогаем при добавлении API

Пакет `mcp/` — универсальный MCP слой. При добавлении нового tool вы меняете только:
- `server.go` — добавить в `buildToolRegistry()`
- `handlers.go` — добавить case в `executeTool()` и реализацию

### `internal/myapi/` — один файл на домен

```
internal/myapi/
├── client.go    ← NewClient, Options, get/post helpers
├── users.go     ← GetUsers, GetUser и типы User, UserResponse
├── orders.go    ← GetOrders и типы
└── ...          ← один файл = одна область данных
```

---

## go.mod

```go
module github.com/username/my-api-mcp

go 1.22

require github.com/joho/godotenv v1.5.1
```

Политика: **минимум зависимостей**. MCP протокол реализован с нуля — без сторонних MCP SDK. Тесты — только стандартный `testing` пакет.

{: .note }
> Запрещено без обсуждения: ORM, big frameworks (gin, echo, cobra), дублирование stdlib.

---

## .gitignore

```gitignore
.env          ← секреты
bin/          ← локальные бинарники
dist/         ← горелизер артефакты
coverage.out
coverage.html
go.work
go.work.sum
```

---

## opencode.jsonc

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "my-api": {
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

{: .important }
> Используйте абсолютный путь к бинарнику. Относительные пути могут не работать в зависимости от рабочей директории opencode.

---

## Что дальше?

- [Протокол и транспорт]({{ site.baseurl }}/protocol) — как работает коммуникация
