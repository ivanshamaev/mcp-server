# Реализация сервера

## Server struct (`internal/mcp/server.go`)

```go title="internal/mcp/server.go"
package mcp

const (
    protocolVersion = "2025-11-25"
    serverName      = "my-api-mcp"  // ← поменяйте на имя вашего сервера
)

type Server struct {
    transport   *StdioTransport
    myapi       *myapi.Client
    logger      *slog.Logger
    version     string
    initialized bool
    tools       []Tool
}

func NewServer(transport *StdioTransport, mc *myapi.Client,
    logger *slog.Logger, version string) *Server {
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
        // Проверить отмену до блокирующего чтения
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
                return nil  // нормальное завершение
            }
            var pe *parseError
            if errors.As(err, &pe) {
                // Невалидный JSON — отправить ошибку и продолжить
                _ = s.transport.WriteResponse(
                    errorResponse(nil, CodeParseError, err.Error()))
                continue
            }
            return fmt.Errorf("transport read: %w", err)
        }

        // Notifications (id == nil) — без ответа
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

### Ключевые решения

| Решение | Почему |
|---------|--------|
| `select` перед `ReadRequest` | Проверяет отмену без блокировки |
| `io.EOF` → `return nil` | Нормальное завершение, не ошибка |
| Parse errors → `continue` | Невалидный JSON не убивает сервер |
| Notifications → без ответа | По протоколу MCP |

---

## Dispatcher

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
```

---

## initialize

```go
func (s *Server) handleInitialize(req *Request) Response {
    var params InitializeParams
    if err := json.Unmarshal(req.Params, &params); err != nil {
        return errorResponse(req.ID, CodeInvalidParams,
            "invalid initialize params: "+err.Error())
    }

    s.initialized = true  // идемпотентно — повторный вызов безопасен

    return okResponse(req.ID, InitializeResult{
        ProtocolVersion: protocolVersion,
        Capabilities: ServerCapabilities{
            Tools: &ToolsCapability{ListChanged: false},
        },
        ServerInfo: ServerInfo{Name: serverName, Version: s.version},
    })
}
```

---

## Точка входа (`cmd/server/main.go`)

```go title="cmd/server/main.go"
package main

import (
    "context"
    "fmt"
    "io"
    "log/slog"
    "os"
    "os/signal"
    "syscall"
)

// Все три должны быть объявлены — Go молча игнорирует -X для несуществующих.
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
    cfg, err := config.Load()
    if err != nil {
        return fmt.Errorf("config: %w", err)
    }

    // Логгер — ТОЛЬКО в stderr или файл, никогда в stdout!
    var logWriter io.Writer = os.Stderr
    if cfg.LogFile != "" {
        f, err := os.OpenFile(cfg.LogFile,
            os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
        if err != nil {
            return fmt.Errorf("open log file: %w", err)
        }
        defer f.Close()
        logWriter = f
    }

    logger := slog.New(slog.NewJSONHandler(logWriter, &slog.HandlerOptions{
        Level: cfg.LogLevel,
    }))

    mc := myapi.NewClient(cfg.AccessToken, myapi.WithBaseURL(cfg.BaseURL))

    transport := mcp.NewStdioTransport(os.Stdin, os.Stdout)
    logger.Info("starting server", "version", version, "commit", commit, "date", date)
    server := mcp.NewServer(transport, mc, logger, version)

    // Graceful shutdown по SIGTERM/Ctrl+C
    ctx, stop := signal.NotifyContext(context.Background(),
        os.Interrupt, syscall.SIGTERM)
    defer stop()

    return server.Run(ctx)
}
```

---

## Конфигурация (`internal/config/config.go`)

```go title="internal/config/config.go"
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
    _ = godotenv.Load()  // .env опционален — ошибка игнорируется

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
```

!!! danger "Имена переменных окружения"
    `godotenv.Load()` загружает ключи точно так, как они записаны в `.env`.
    
    ```bash
    # ❌ Неправильно — код читает ACCESS_TOKEN, но в .env написано:
    AccessToken=y0_...
    
    # ✅ Правильно
    ACCESS_TOKEN=y0_...
    ```
    
    Несовпадение имён = токен не найден = ошибка старта.
