package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ivshamaev/yametrika-mcp/internal/config"
	"github.com/ivshamaev/yametrika-mcp/internal/mcp"
	"github.com/ivshamaev/yametrika-mcp/internal/metrika"
)

// version is set at build time via -ldflags.
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// ── Config ──────────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	// ── Logger ──────────────────────────────────────────────────────────────
	// IMPORTANT: logs must NEVER go to stdout — that's reserved for JSON-RPC.
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

	// ── Metrika client ───────────────────────────────────────────────────────
	mc := metrika.NewClient(cfg.AccessToken,
		metrika.WithBaseURL(cfg.MetrikaBaseURL),
	)

	// ── MCP transport + server ───────────────────────────────────────────────
	transport := mcp.NewStdioTransport(os.Stdin, os.Stdout)
	server := mcp.NewServer(transport, mc, logger, version)

	// ── Graceful shutdown ────────────────────────────────────────────────────
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return server.Run(ctx)
}
