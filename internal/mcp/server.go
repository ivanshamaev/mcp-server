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
		Instructions: `Yandex Metrika MCP Server — доступ к аналитике сайтов через Yandex Metrika API.

ТИПИЧНЫЕ СЦЕНАРИИ:
- Трафик за период: metrika_get_report(metrics="ym:s:visits,ym:s:users", dimensions="ym:s:date")
- Топ страниц: metrika_get_report(metrics="ym:s:visits", dimensions="ym:s:startURL", sort="-ym:s:visits")
- Поисковые запросы: metrika_get_report(metrics="ym:s:visits", dimensions="ym:s:lastSignSearchPhrase", sort="-ym:s:visits")
- Источники трафика: metrika_get_report(metrics="ym:s:visits", dimensions="ym:s:trafficSourceName,ym:s:searchEngineName")
- Трафик на конкретный раздел: добавь filters="ym:s:startURL=@'/раздел'"

ВАЖНО:
- Для поисковых фраз используй ym:s:lastSignSearchPhrase (не ym:s:searchPhrase).
- date1/date2: YYYY-MM-DD или ключевые слова today, yesterday, 7daysAgo, 30daysAgo, 90daysAgo.
- Фильтры: == (равно), != (не равно), =@ (содержит), AND/OR для составных условий.
- Logs API workflow: create_log_request → polling get_log_request(status='processed') → download_log → clean_log_request.`,
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
			Description: "Получить список всех счётчиков Yandex Metrika аккаунта. Используй, чтобы узнать доступные counter_id перед другими запросами. Возвращает id, name, site URL для каждого счётчика.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		{
			Name:        "metrika_get_counter",
			Description: "Получить подробную информацию о конкретном счётчике: статус, владелец, сайт. Используй, чтобы проверить что counter_id существует и доступен.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"counter_id": {Type: "string", Description: "ID счётчика Yandex Metrika"},
				},
				Required: []string{"counter_id"},
			},
		},
		{
			Name: "metrika_get_report",
			Description: `Получить статистический отчёт по метрикам и измерениям (Reports API /stat/v1/data).
Используй для анализа трафика, конверсий, источников, страниц, поисковых запросов за период.

МЕТРИКИ (metrics) — через запятую:
  ym:s:visits — визиты, ym:s:users — уникальные пользователи,
  ym:s:pageviews — просмотры страниц, ym:s:bounceRate — отказы (%),
  ym:s:avgVisitDurationSeconds — средняя длительность визита

ИЗМЕРЕНИЯ (dimensions) — через запятую:
  ym:s:date — дата | ym:s:startURL — URL входа | ym:s:lastSignSearchPhrase — поисковая фраза,
  приведшая к визиту (используй вместо ym:s:searchPhrase для актуальных данных) |
  ym:s:trafficSourceName — источник трафика | ym:s:searchEngineName — поисковик |
  ym:s:regionCityName — город | ym:s:deviceCategory — устройство (desktop/mobile/tablet)

ФИЛЬТРЫ (filters) — синтаксис Metrika Filter:
  ym:s:startURL=@'pyspark'       — URL содержит 'pyspark' (оператор =@)
  ym:s:regionCityName=='Москва'  — точное совпадение (оператор ==)
  ym:s:searchEngineName!='None'  — исключение
  Составные: condition1 AND condition2 | condition1 OR condition2`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"counter_id": {Type: "string", Description: "ID счётчика"},
					"metrics":    {Type: "string", Description: "Метрики через запятую, напр. ym:s:visits,ym:s:users"},
					"dimensions": {Type: "string", Description: "Измерения через запятую, напр. ym:s:date,ym:s:lastSignSearchPhrase"},
					"date1":      {Type: "string", Description: "Начало периода: YYYY-MM-DD или 7daysAgo, 30daysAgo, today, yesterday", Default: "7daysAgo"},
					"date2":      {Type: "string", Description: "Конец периода: YYYY-MM-DD или today, yesterday", Default: "today"},
					"sort":       {Type: "string", Description: "Поле сортировки; префикс '-' для убывания, напр. -ym:s:visits"},
					"limit":      {Type: "integer", Description: "Максимум строк в ответе (1–100000)", Default: 100},
					"filters":    {Type: "string", Description: "Фильтр Metrika, напр. ym:s:startURL=@'pyspark' AND ym:s:searchEngineName!='None'"},
					"group":      {Type: "string", Description: "Группировка по времени: day, week, month"},
				},
				Required: []string{"counter_id", "metrics"},
			},
		},
		{
			Name:        "metrika_get_goals",
			Description: "Получить список целей счётчика с их ID, названиями и условиями. Используй, чтобы узнать goal_id для фильтрации отчётов по конверсиям.",
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
			Description: "Получить список сегментов аудитории счётчика. Используй, чтобы узнать доступные сегменты для фильтрации отчётов.",
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
			Description: "Получить список всех запросов на выгрузку логов для счётчика. Используй для просмотра существующих запросов и их статусов (created/processed/cleaned).",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"counter_id": {Type: "string", Description: "ID счётчика"},
				},
				Required: []string{"counter_id"},
			},
		},
		{
			Name:        "metrika_get_log_request",
			Description: "Получить статус конкретного запроса на выгрузку логов по его ID. Используй для проверки готовности: статус 'processed' означает что данные готовы к скачиванию. Статусы: created → processed → cleaned.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"counter_id": {Type: "string", Description: "ID счётчика"},
					"request_id": {Type: "string", Description: "ID запроса логов (из metrika_create_log_request или metrika_list_logs)"},
				},
				Required: []string{"counter_id", "request_id"},
			},
		},
		{
			Name:        "metrika_create_log_request",
			Description: "Создать запрос на выгрузку сырых логов посещений (visits) или хитов (hits). После создания проверяй статус через metrika_get_log_request, пока не станет 'processed', затем скачивай через metrika_download_log.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"counter_id": {Type: "string", Description: "ID счётчика"},
					"fields":     {Type: "string", Description: "Поля через запятую, напр. ym:s:visitID,ym:s:date,ym:s:pageViews,ym:s:clientID"},
					"source": {
						Type:        "string",
						Description: "Тип данных: visits (визиты) или hits (хиты/просмотры страниц)",
						Enum:        []string{"visits", "hits"},
					},
					"date1": {Type: "string", Description: "Начало периода YYYY-MM-DD (не раньше 90 дней назад)"},
					"date2": {Type: "string", Description: "Конец периода YYYY-MM-DD"},
				},
				Required: []string{"counter_id", "fields", "source", "date1", "date2"},
			},
		},
		{
			Name:        "metrika_download_log",
			Description: "Скачать часть выгруженных логов в формате TSV (после того как статус запроса стал 'processed'). Возвращает TSV-текст с заголовком в первой строке. Если логов несколько частей — перебирай part_number начиная с 0.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"counter_id":  {Type: "string", Description: "ID счётчика"},
					"request_id":  {Type: "string", Description: "ID запроса логов"},
					"part_number": {Type: "integer", Description: "Номер части (начиная с 0)", Default: 0},
				},
				Required: []string{"counter_id", "request_id"},
			},
		},
		{
			Name:        "metrika_clean_log_request",
			Description: "Удалить завершённый запрос логов, освободив слот. Metrika ограничивает число одновременных запросов на счётчик — вызывай после скачивания всех частей.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"counter_id": {Type: "string", Description: "ID счётчика"},
					"request_id": {Type: "string", Description: "ID запроса логов для удаления"},
				},
				Required: []string{"counter_id", "request_id"},
			},
		},
	}
}
