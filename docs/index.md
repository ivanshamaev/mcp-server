---
layout: default
title: Главная
nav_order: 1
description: "Руководство по созданию MCP сервера на Go для opencode"
permalink: /
---

# Go MCP Server — Developer Guide
{: .fs-9 }

Полное руководство по созданию MCP (Model Context Protocol) сервера на Go для любого API — от нуля до production.
{: .fs-6 .fw-300 }

[Начать →]({{ site.baseurl }}/overview){: .btn .btn-primary .fs-5 .mb-4 .mb-md-0 .mr-2 }
[GitHub](https://github.com/ivanshamaev/mcp-server){: .btn .fs-5 .mb-4 .mb-md-0 }

---

## Что это?

Это практическое руководство по созданию MCP сервера на Go, основанное на реальном проекте [yametrika-mcp](https://github.com/ivanshamaev/mcp-server) — интеграции Yandex Metrika с AI-редакторами через открытый протокол MCP.

MCP (Model Context Protocol) позволяет AI-помощникам (Claude Code, opencode и другим) напрямую обращаться к внешним API через стандартизированный интерфейс инструментов (tools).

## Что вы получите

После изучения этого руководства вы сможете создать MCP сервер для **любого REST API**, который будет:

- ✅ Работать через stdio JSON-RPC 2.0 (стандарт MCP)
- ✅ Подключаться к opencode / Claude Code / любому MCP-клиенту
- ✅ Собираться для Linux, macOS (Intel + Silicon), Windows
- ✅ Иметь CI/CD с автоматическими релизами через goreleaser

## Разделы руководства

| Раздел | Описание |
|--------|----------|
| [Обзор и концепции]({{ site.baseurl }}/overview) | Что такое MCP, архитектура, протокол |
| [Предварительные требования]({{ site.baseurl }}/prerequisites) | Установка Go, инструментов |
| [Структура проекта]({{ site.baseurl }}/project-structure) | Пакеты, файлы, принципы организации |
| [Протокол и транспорт]({{ site.baseurl }}/protocol) | JSON-RPC 2.0, stdio транспорт, lifecycle |
| [Реализация сервера]({{ site.baseurl }}/server) | Server, типы, handlers |
| [HTTP клиент]({{ site.baseurl }}/api-client) | Паттерны для любого REST API |
| [Добавление инструментов]({{ site.baseurl }}/adding-tools) | Пошаговый процесс добавления tool |
| [Тестирование]({{ site.baseurl }}/testing) | Unit тесты, E2E тесты протокола |
| [CI/CD и релизы]({{ site.baseurl }}/cicd) | GitHub Actions, goreleaser, versioning |
| [Решение проблем]({{ site.baseurl }}/troubleshooting) | Типичные ошибки и их исправление |

## Быстрый старт

```bash
# 1. Создать проект
mkdir my-api-mcp && cd my-api-mcp
git init
export PATH="$HOME/.local/go/bin:$PATH"
go mod init github.com/username/my-api-mcp

# 2. Скопировать шаблон MCP слоя
mkdir -p cmd/server internal/{mcp,myapi,config}

# 3. Реализовать (следуй разделу "Реализация сервера")

# 4. Собрать и протестировать
go build -o bin/mcp-server ./cmd/server
go test ./... -v

# 5. E2E тест
printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{...}}\n' \
  | ./bin/mcp-server
```

## Технический стек

```
Go 1.22+
└── stdlib only (encoding/json, net/http, log/slog, ...)
    └── github.com/joho/godotenv (загрузка .env)

MCP протокол: 2024-11-05 (stable)
Transport: stdio JSON-RPC 2.0 (newline-delimited)

CI/CD:
├── GitHub Actions (test × 3 платформы, lint, build × 5 платформ)
└── goreleaser v2 (кросс-компиляция, GitHub Releases)
```
