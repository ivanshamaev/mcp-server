---
layout: default
title: Реализация сервера
nav_order: 6
permalink: /server
---

# Реализация сервера
{: .no_toc }

<details open markdown="block">
  <summary>Содержание</summary>
  {: .text-delta }
1. TOC
{:toc}
</details>

---

## Server struct (`internal/mcp/server.go`)

```go
package mcp

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "log/slog"
)

const (
    protocolVersion = "2024-11-05"
    serverName      = "my-api-mcp"  // ← поменяйте на имя вашего сервера
)

type Server struct {
    transport   *StdioTransport
    myapi       *myapi.Client    // ← ваш API клиент
    logger      *slog.Logger
    version     string
    initialized bool
    tools       []Tool
}

func NewServer(transport *StdioTransport, mc *myapi.Client, logger *slog.Logger, version string) *Server {
    s := &Server{
        transport: transport,
        myapi:     mc,
        logger:    logger,
        version:   version,
    }
    s.tools = s.buildToolRegistry()
    return s
}
```

---

## Main loop

```go
func (s *Server) Run(ctx context.Context) error {
    s.logger.Info("MCP server started", "version", s.version, "protocol", protocolVersion)

    for {
        // Проверить отмену контекста перед блокирующим чтением
        select {
        case <-ctx.Done():
            s.logger.Info("server shutting down", "reason", ctx.Err())
            return nil
        default:
        }

        req, err := s.transport.ReadRequest()
        if err != nil {
            if errors.Is(err, io.EOF) {
                s.logger.Info("stdin closed, exiting")
                return nil
            }
            // Parse errors — отправить ответ с ошибкой и продолжить
            var pe *parseError
            if errors.As(err, &pe) {
                _ = s.transport.WriteResponse(errorResponse(nil, CodeParseError, err.Error()))
                continue
            }
            return fmt.Errorf("transport read: %w", err)
        }

        // Notifications (id == nil) — обработать без ответа
        if req.ID == nil {
            s.handleNotification(ctx, req)
            continue
        }

        resp := s.handleRequest(ctx, req)
        if err := s.transport.WriteResponse(resp); err != nil {
            return fmt.Errorf("transport write: %w", err)
        }
    }
}
```

### Ключевые решения в main loop

**`select` перед `ReadRequest`** — проверяет отмену контекста без блокировки. Без этого сервер не остановится по SIGTERM пока stdin не закрыт.

**`io.EOF` — нормальное завершение** — когда клиент закрывает stdin, сервер корректно завершается.

**Parse errors — `continue`** — невалидный JSON не должен убивать сервер. Отправляем ответ с ошибкой и ждём следующего сообщения.

---

## Обработка запросов

```go
func (s *Server) handleRequest(ctx context.Context, req *Request) Response {
    switch req.Method {
    case "initialize":
        return s.handleInitialize(req)
    case "tools/list":
        return s.handleToolsList(req)
    case "tools/call":
        return s.handleToolsCall(ctx, req)
    case "ping":
        return okResponse(req.ID, struct{}{})
    default:
        return errorResponse(req.ID, CodeMethodNotFound, "method not found: "+req.Method)
    }
}

func (s *Server) handleNotification(_ context.Context, req *Request) {
    switch req.Method {
    case "notifications/initialized":
        s.logger.Info("client initialized")
    default:
        s.logger.Debug("unknown notification", "method", req.Method)
    }
}
```

---

## initialize

```go
func (s *Server) handleInitialize(req *Request) Response {
    var params InitializeParams
    if err := json.Unmarshal(req.Params, &params); err != nil {
        return errorResponse(req.ID, CodeInvalidParams, "invalid initialize params: "+err.Error())
    }

    s.logger.Info("initialize",
        "clientName", params.ClientInfo.Name,
        "clientVersion", params.ClientInfo.Version,
    )

    s.initialized = true

    return okResponse(req.ID, InitializeResult{
        ProtocolVersion: protocolVersion,
        Capabilities: ServerCapabilities{
            Tools: &ToolsCapability{ListChanged: false},
        },
        ServerInfo: ServerInfo{
            Name:    serverName,
            Version: s.version,
        },
    })
}
```

{: .note }
> `initialized = true` устанавливается один раз. Повторный `initialize` просто перезапишет флаг — это идемпотентно и безопасно.

---

## tools/list и tools/call

```go
func (s *Server) handleToolsList(req *Request) Response {
    if !s.initialized {
        return errorResponse(req.ID, CodeInvalidRequest, "server not initialized")
    }
    return okResponse(req.ID, toolsListResult{Tools: s.tools})
}

func (s *Server) handleToolsCall(ctx context.Context, req *Request) Response {
    if !s.initialized {
        return errorResponse(req.ID, CodeInvalidRequest, "server not initialized")
    }

    var params ToolCallParams
    if err := json.Unmarshal(req.Params, &params); err != nil {
        return errorResponse(req.ID, CodeInvalidParams, "invalid tools/call params: "+err.Error())
    }

    s.logger.Info("tool call", "tool", params.Name, "args", params.Arguments)

    result := s.executeTool(ctx, params.Name, params.Arguments)
    return okResponse(req.ID, result)  // ← ВСЕГДА okResponse, даже если isError: true
}
```

---

## Точка входа (`cmd/server/main.go`)

```go
package main

import (
    "context"
    "fmt"
    "io"
    "log/slog"
    "os"
    "os/signal"
    "syscall"

    "github.com/username/my-api-mcp/internal/config"
    "github.com/username/my-api-mcp/internal/mcp"
    "github.com/username/my-api-mcp/internal/myapi"
)

// Переменные инжектируются goreleaser через -ldflags.
// ВАЖНО: все три должны быть объявлены — Go молча игнорирует -X для несуществующих.
var (
    version = "dev"
    commit  = "none"
    date    = "unknown"
)

func main() {
    if err := run(); err != nil {
        fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
        os.Exit(1)
    }
}

func run() error {
    // Конфигурация
    cfg, err := config.Load()
    if err != nil {
        return fmt.Errorf("config: %w", err)
    }

    // Логгер — ТОЛЬКО в stderr или файл, никогда в stdout!
    var logWriter io.Writer = os.Stderr
    if cfg.LogFile != "" {
        f, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
        if err != nil {
            return fmt.Errorf("open log file: %w", err)
        }
        defer f.Close()
        logWriter = f
    }

    logger := slog.New(slog.NewJSONHandler(logWriter, &slog.HandlerOptions{
        Level: cfg.LogLevel,
    }))
    slog.SetDefault(logger)

    // API клиент
    mc := myapi.NewClient(cfg.AccessToken,
        myapi.WithBaseURL(cfg.BaseURL),
    )

    // MCP сервер
    transport := mcp.NewStdioTransport(os.Stdin, os.Stdout)
    logger.Info("starting server", "version", version, "commit", commit, "date", date)
    server := mcp.NewServer(transport, mc, logger, version)

    // Graceful shutdown по SIGTERM/Ctrl+C
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    return server.Run(ctx)
}
```

---

## Конфигурация (`internal/config/config.go`)

```go
package config

import (
    "fmt"
    "log/slog"
    "os"

    "github.com/joho/godotenv"
)

type Config struct {
    AccessToken string
    BaseURL     string
    LogLevel    slog.Level
    LogFile     string
}

func Load() (*Config, error) {
    // Загрузить .env если существует (ошибка игнорируется — .env опционален)
    _ = godotenv.Load()

    token := os.Getenv("ACCESS_TOKEN")
    if token == "" {
        return nil, fmt.Errorf("ACCESS_TOKEN is required")
    }

    cfg := &Config{
        AccessToken: token,
        BaseURL:     getEnvDefault("BASE_URL", "https://api.example.com"),
        LogFile:     os.Getenv("LOG_FILE"),
        LogLevel:    slog.LevelInfo,
    }

    if lvl := os.Getenv("LOG_LEVEL"); lvl != "" {
        if err := cfg.LogLevel.UnmarshalText([]byte(lvl)); err != nil {
            return nil, fmt.Errorf("invalid LOG_LEVEL %q: %w", lvl, err)
        }
    }

    return cfg, nil
}

func getEnvDefault(key, def string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return def
}
```

{: .important }
> **Имена переменных окружения**: `godotenv.Load()` загружает ключи точно так, как они записаны в `.env`. Если `.env` содержит `AccessToken=`, а код читает `os.Getenv("ACCESS_TOKEN")` — токен не найден! Всегда используйте `UPPER_SNAKE_CASE`.

---

## Что дальше?

- [HTTP клиент]({{ site.baseurl }}/api-client) — паттерны для REST API
- [Добавление инструментов]({{ site.baseurl }}/adding-tools) — пошаговый процесс
