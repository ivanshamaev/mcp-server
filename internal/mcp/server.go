package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/ivshamaev/yametrika-mcp/internal/metrika"
)

const (
	protocolVersion = "2025-11-25"
	serverName      = "yandex-metrika-mcp"
)

// Server is the MCP server. It reads requests from the transport,
// dispatches them to handler methods, and writes responses back.
type Server struct {
	transport   *StdioTransport
	metrika     *metrika.Client
	logger      *slog.Logger
	version     string
	initialized bool
	tools       []Tool
}

// NewServer creates a Server wired to the given transport and Metrika client.
func NewServer(transport *StdioTransport, mc *metrika.Client, logger *slog.Logger, version string) *Server {
	s := &Server{
		transport: transport,
		metrika:   mc,
		logger:    logger,
		version:   version,
	}
	s.tools = s.buildToolRegistry()
	return s
}

// Run starts the main request-response loop. It blocks until stdin is closed
// or ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	s.logger.Info("MCP server started", "version", s.version, "protocol", protocolVersion)

	for {
		// Check for context cancellation before blocking on read.
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

			// Parse errors: send error response (we have no id).
			var pe *parseError
			if errors.As(err, &pe) {
				s.logger.Warn("parse error", "err", err)
				_ = s.transport.WriteResponse(errorResponse(nil, CodeParseError, err.Error()))
				continue
			}

			var ie *invalidRequestError
			if errors.As(err, &ie) {
				s.logger.Warn("invalid request", "err", err)
				_ = s.transport.WriteResponse(errorResponse(nil, CodeInvalidRequest, err.Error()))
				continue
			}

			return fmt.Errorf("transport read: %w", err)
		}

		s.logger.Debug("received request", "method", req.Method, "id", req.ID)

		// Notifications have no id — handle and don't reply.
		if req.ID == nil {
			s.handleNotification(ctx, req)
			continue
		}

		resp := s.handleRequest(ctx, req)
		if err := s.transport.WriteResponse(resp); err != nil {
			return fmt.Errorf("transport write: %w", err)
		}
		s.logger.Debug("sent response", "id", req.ID)
	}
}

// handleNotification processes JSON-RPC notifications (no response expected).
func (s *Server) handleNotification(_ context.Context, req *Request) {
	switch req.Method {
	case "notifications/initialized":
		s.logger.Info("client initialized")
	default:
		s.logger.Debug("unknown notification", "method", req.Method)
	}
}

// handleRequest dispatches a request to the appropriate handler.
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

// ─── handlers ────────────────────────────────────────────────────────────────

func (s *Server) handleInitialize(req *Request) Response {
	var params InitializeParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResponse(req.ID, CodeInvalidParams, "invalid initialize params: "+err.Error())
	}

	s.logger.Info("initialize",
		"clientName", params.ClientInfo.Name,
		"clientVersion", params.ClientInfo.Version,
		"clientProtocol", params.ProtocolVersion,
	)
	// Log a warning if client requested a different protocol version.
	// Server always responds with its own supported version (2025-11-25);
	// per MCP spec the client should disconnect if it cannot accept our version.
	if params.ProtocolVersion != protocolVersion {
		s.logger.Warn("client requested different protocol version",
			"clientVersion", params.ProtocolVersion,
			"serverVersion", protocolVersion,
		)
	}

	s.initialized = true

	result := InitializeResult{
		ProtocolVersion: protocolVersion,
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{ListChanged: false},
		},
		ServerInfo: ServerInfo{
			Name:    serverName,
			Version: s.version,
		},
	}
	return okResponse(req.ID, result)
}

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
	return okResponse(req.ID, result)
}

// ─── tool registry ────────────────────────────────────────────────────────────

// buildToolRegistry returns all tools this server exposes.
func (s *Server) buildToolRegistry() []Tool {
	return []Tool{
		{
			Name:        "metrika_get_counters",
			Description: "Получить список всех счётчиков Yandex Metrika аккаунта",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		{
			Name:        "metrika_get_counter",
			Description: "Получить подробную информацию о конкретном счётчике",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"counter_id": {Type: "string", Description: "ID счётчика Yandex Metrika"},
				},
				Required: []string{"counter_id"},
			},
		},
		{
			Name:        "metrika_get_report",
			Description: "Получить статистический отчёт по метрикам и измерениям (Reports API)",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"counter_id": {Type: "string", Description: "ID счётчика"},
					"metrics":    {Type: "string", Description: "Метрики через запятую, напр. ym:s:visits,ym:s:pageviews"},
					"dimensions": {Type: "string", Description: "Измерения через запятую, напр. ym:s:date,ym:s:sourceEngine"},
					"date1":      {Type: "string", Description: "Начало периода YYYY-MM-DD или 7daysAgo, today и т.д.", Default: "7daysAgo"},
					"date2":      {Type: "string", Description: "Конец периода YYYY-MM-DD или today", Default: "today"},
					"sort":       {Type: "string", Description: "Поле сортировки, напр. -ym:s:visits"},
					"limit":      {Type: "string", Description: "Максимум строк в отчёте (1-100000)", Default: "100"},
					"filters":    {Type: "string", Description: "Фильтры в формате Metrika Filter, напр. ym:s:regionCity=='Москва'"},
					"group":      {Type: "string", Description: "Группировка: day, week, month"},
				},
				Required: []string{"counter_id", "metrics"},
			},
		},
		{
			Name:        "metrika_get_goals",
			Description: "Получить список целей счётчика",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"counter_id": {Type: "string", Description: "ID счётчика"},
				},
				Required: []string{"counter_id"},
			},
		},
		{
			Name:        "metrika_get_segments",
			Description: "Получить список сегментов счётчика",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"counter_id": {Type: "string", Description: "ID счётчика"},
				},
				Required: []string{"counter_id"},
			},
		},
		{
			Name:        "metrika_list_logs",
			Description: "Получить список запросов на выгрузку логов (Logs API)",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"counter_id": {Type: "string", Description: "ID счётчика"},
				},
				Required: []string{"counter_id"},
			},
		},
		{
			Name:        "metrika_create_log_request",
			Description: "Создать запрос на выгрузку сырых логов посещений или хитов",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"counter_id": {Type: "string", Description: "ID счётчика"},
					"fields":     {Type: "string", Description: "Поля через запятую, напр. ym:s:visitID,ym:s:date,ym:s:pageViews"},
					"source": {
						Type:        "string",
						Description: "Тип данных: visits (визиты) или hits (хиты)",
						Enum:        []string{"visits", "hits"},
					},
					"date1": {Type: "string", Description: "Начало периода YYYY-MM-DD"},
					"date2": {Type: "string", Description: "Конец периода YYYY-MM-DD"},
				},
				Required: []string{"counter_id", "fields", "source", "date1", "date2"},
			},
		},
		{
			Name:        "metrika_download_log",
			Description: "Скачать часть выгруженных логов (после создания запроса и ожидания его выполнения)",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"counter_id":  {Type: "string", Description: "ID счётчика"},
					"request_id":  {Type: "string", Description: "ID запроса логов"},
					"part_number": {Type: "string", Description: "Номер части (начиная с 0)", Default: "0"},
				},
				Required: []string{"counter_id", "request_id"},
			},
		},
	}
}
