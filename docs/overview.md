---
layout: default
title: Обзор и концепции
nav_order: 2
permalink: /overview
---

# Обзор и концепции
{: .no_toc }

<details open markdown="block">
  <summary>Содержание</summary>
  {: .text-delta }
1. TOC
{:toc}
</details>

---

## Что такое MCP?

**MCP (Model Context Protocol)** — открытый протокол от Anthropic, стандартизирующий коммуникацию между AI-приложениями и внешними системами.

```
┌──────────────────┐     MCP Protocol      ┌──────────────────┐
│   MCP Client     │ ◄───────────────────► │   MCP Server     │
│  (opencode,      │   JSON-RPC 2.0        │  (ваш сервер)    │
│   Claude Code)   │   over stdio          │                  │
└──────────────────┘                       └──────────────────┘
                                                    │
                                                    │ HTTP/REST
                                                    ▼
                                           ┌──────────────────┐
                                           │  Внешний API     │
                                           │  (Metrika,       │
                                           │   GitHub, ...)   │
                                           └──────────────────┘
```

MCP сервер предоставляет **инструменты (tools)** — функции, которые AI-клиент может вызывать для получения данных или выполнения действий.

## Зачем это нужно?

Без MCP AI-помощник не может напрямую обращаться к вашим API. С MCP сервером вы даёте AI доступ к любым данным:

```
Пользователь: "Покажи топ-5 источников трафика за прошлую неделю"
       ↓
Claude (opencode) вызывает: metrika_get_report(counter_id=..., metrics=visits, date1=7daysAgo)
       ↓
MCP сервер делает HTTP запрос к Yandex Metrika API
       ↓
Возвращает данные → Claude анализирует → Отвечает пользователю
```

## Транспорт: stdio JSON-RPC

MCP использует **stdio транспорт**: сервер читает из `stdin` и пишет в `stdout` JSON-RPC сообщения, разделённые переводом строки.

```
stdin  → [JSON-RPC request]\n  → сервер
stdout ← [JSON-RPC response]\n ← сервер
stderr → логи (только сюда! stdout зарезервирован)
```

**Критически важно:** любой вывод не-JSON в `stdout` сломает протокол. Все логи — только в `stderr` или файл.

## Lifecycle соединения

```
Client                          Server
  │                               │
  │── initialize ────────────────►│  Handshake: клиент представляется
  │◄─ InitializeResult ───────────│  Сервер отвечает своими capabilities
  │                               │
  │── notifications/initialized ─►│  Уведомление: "готов к работе"
  │                               │
  │── tools/list ────────────────►│  Запрос списка инструментов
  │◄─ {tools: [...]} ─────────────│
  │                               │
  │── tools/call ────────────────►│  Вызов инструмента
  │◄─ ToolCallResult ─────────────│
  │                               │
  │   (повторяется N раз)         │
  │                               │
  │── EOF ───────────────────────►│  Конец работы
```

## Структура JSON-RPC сообщений

**Request** (от клиента к серверу):
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "myapi_get_items",
    "arguments": {"limit": "10"}
  }
}
```

**Response** (от сервера к клиенту):
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [{"type": "text", "text": "[{\"id\": 1, ...}]"}],
    "isError": false
  }
}
```

**Notification** (без id, ответ не нужен):
```json
{
  "jsonrpc": "2.0",
  "method": "notifications/initialized"
}
```

## Два вида ошибок

Это ключевое различие MCP:

| Тип ошибки | Когда | Формат |
|-----------|-------|--------|
| **Протокольная** | Неверный JSON, неизвестный метод, не инициализировано | `{"error": {"code": -32601, "message": "..."}}` |
| **Tool error** | API вернул ошибку, неверные параметры tool | `{"result": {"content": [...], "isError": true}}` |

```
HTTP 404 от API → toolGetCounter возвращает errorContent("...") 
→ Server.handleToolsCall оборачивает в okResponse (не errorResponse!)
→ {"result": {"content": [...], "isError": true}}
```

Клиент (opencode) покажет tool error как сообщение об ошибке, а не как сбой протокола.

## Версия протокола

Используем **`2024-11-05`** — стабильная версия, поддерживаемая opencode и Claude Code.

Сервер всегда отвечает этой же версией в `initialize`, независимо от того, что прислал клиент.

## Что дальше?

- [Предварительные требования]({{ site.baseurl }}/prerequisites) — установить Go и инструменты
- [Структура проекта]({{ site.baseurl }}/project-structure) — как организован код
