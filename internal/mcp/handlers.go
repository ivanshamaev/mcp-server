package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

// executeTool dispatches a tools/call to the correct Metrika method.
func (s *Server) executeTool(ctx context.Context, name string, args map[string]any) ToolCallResult {
	switch name {
	case "metrika_get_counters":
		return s.toolGetCounters(ctx)
	case "metrika_get_counter":
		return s.toolGetCounter(ctx, args)
	case "metrika_get_report":
		return s.toolGetReport(ctx, args)
	case "metrika_get_goals":
		return s.toolGetGoals(ctx, args)
	case "metrika_get_segments":
		return s.toolGetSegments(ctx, args)
	case "metrika_list_logs":
		return s.toolListLogs(ctx, args)
	case "metrika_get_log_request":
		return s.toolGetLogRequest(ctx, args)
	case "metrika_create_log_request":
		return s.toolCreateLogRequest(ctx, args)
	case "metrika_download_log":
		return s.toolDownloadLog(ctx, args)
	case "metrika_clean_log_request":
		return s.toolCleanLogRequest(ctx, args)
	default:
		return errorContent(fmt.Sprintf("unknown tool: %s", name))
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func getString(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func getStringDefault(args map[string]any, key, def string) string {
	if v, ok := args[key].(string); ok && v != "" {
		return v
	}
	return def
}

// getIntString returns an integer parameter as a string, handling both JSON
// number (float64) and string inputs. LLMs often pass numeric params as numbers.
func getIntString(args map[string]any, key, def string) string {
	switch v := args[key].(type) {
	case string:
		if v != "" {
			return v
		}
	case float64:
		return fmt.Sprintf("%.0f", v)
	}
	return def
}

func jsonText(v any) (ToolCallResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return errorContent("failed to serialize response: " + err.Error()), err
	}
	return textContent(string(b)), nil
}

// ─── tool implementations ────────────────────────────────────────────────────

func (s *Server) toolGetCounters(ctx context.Context) ToolCallResult {
	counters, err := s.metrika.GetCounters(ctx)
	if err != nil {
		return errorContent("Ошибка получения счётчиков: " + err.Error())
	}
	result, _ := jsonText(counters)
	return result
}

func (s *Server) toolGetCounter(ctx context.Context, args map[string]any) ToolCallResult {
	id := getString(args, "counter_id")
	if id == "" {
		return errorContent("параметр counter_id обязателен")
	}
	counter, err := s.metrika.GetCounter(ctx, id)
	if err != nil {
		return errorContent(fmt.Sprintf("Ошибка получения счётчика %s: %s", id, err))
	}
	result, _ := jsonText(counter)
	return result
}

func (s *Server) toolGetReport(ctx context.Context, args map[string]any) ToolCallResult {
	counterID := getString(args, "counter_id")
	if counterID == "" {
		return errorContent("параметр counter_id обязателен")
	}
	metrics := getString(args, "metrics")
	if metrics == "" {
		return errorContent("параметр metrics обязателен")
	}

	params := map[string]string{
		"id":      counterID,
		"metrics": metrics,
		"date1":   getStringDefault(args, "date1", "7daysAgo"),
		"date2":   getStringDefault(args, "date2", "today"),
		"limit":   getIntString(args, "limit", "100"),
	}
	if v := getString(args, "dimensions"); v != "" {
		params["dimensions"] = v
	}
	if v := getString(args, "sort"); v != "" {
		params["sort"] = v
	}
	if v := getString(args, "filters"); v != "" {
		params["filters"] = v
	}
	if v := getString(args, "group"); v != "" {
		params["group"] = v
	}

	report, err := s.metrika.GetReport(ctx, params)
	if err != nil {
		return errorContent("Ошибка получения отчёта: " + err.Error())
	}
	result, _ := jsonText(report)
	return result
}

func (s *Server) toolGetGoals(ctx context.Context, args map[string]any) ToolCallResult {
	id := getString(args, "counter_id")
	if id == "" {
		return errorContent("параметр counter_id обязателен")
	}
	goals, err := s.metrika.GetGoals(ctx, id)
	if err != nil {
		return errorContent(fmt.Sprintf("Ошибка получения целей счётчика %s: %s", id, err))
	}
	result, _ := jsonText(goals)
	return result
}

func (s *Server) toolGetSegments(ctx context.Context, args map[string]any) ToolCallResult {
	id := getString(args, "counter_id")
	if id == "" {
		return errorContent("параметр counter_id обязателен")
	}
	segments, err := s.metrika.GetSegments(ctx, id)
	if err != nil {
		return errorContent(fmt.Sprintf("Ошибка получения сегментов счётчика %s: %s", id, err))
	}
	result, _ := jsonText(segments)
	return result
}

func (s *Server) toolListLogs(ctx context.Context, args map[string]any) ToolCallResult {
	id := getString(args, "counter_id")
	if id == "" {
		return errorContent("параметр counter_id обязателен")
	}
	logs, err := s.metrika.ListLogs(ctx, id)
	if err != nil {
		return errorContent(fmt.Sprintf("Ошибка получения логов счётчика %s: %s", id, err))
	}
	result, _ := jsonText(logs)
	return result
}

func (s *Server) toolCreateLogRequest(ctx context.Context, args map[string]any) ToolCallResult {
	id := getString(args, "counter_id")
	fields := getString(args, "fields")
	source := getString(args, "source")
	date1 := getString(args, "date1")
	date2 := getString(args, "date2")

	for name, val := range map[string]string{
		"counter_id": id, "fields": fields, "source": source,
		"date1": date1, "date2": date2,
	} {
		if val == "" {
			return errorContent(fmt.Sprintf("параметр %s обязателен", name))
		}
	}

	logReq, err := s.metrika.CreateLogRequest(ctx, id, fields, source, date1, date2)
	if err != nil {
		return errorContent("Ошибка создания запроса логов: " + err.Error())
	}
	result, _ := jsonText(logReq)
	return result
}

func (s *Server) toolGetLogRequest(ctx context.Context, args map[string]any) ToolCallResult {
	counterID := getString(args, "counter_id")
	requestID := getString(args, "request_id")
	if counterID == "" || requestID == "" {
		return errorContent("параметры counter_id и request_id обязательны")
	}
	logReq, err := s.metrika.GetLogRequest(ctx, counterID, requestID)
	if err != nil {
		return errorContent(fmt.Sprintf("Ошибка получения запроса логов %s/%s: %s", counterID, requestID, err))
	}
	result, _ := jsonText(logReq)
	return result
}

func (s *Server) toolCleanLogRequest(ctx context.Context, args map[string]any) ToolCallResult {
	counterID := getString(args, "counter_id")
	requestID := getString(args, "request_id")
	if counterID == "" || requestID == "" {
		return errorContent("параметры counter_id и request_id обязательны")
	}
	logReq, err := s.metrika.CleanLogRequest(ctx, counterID, requestID)
	if err != nil {
		return errorContent(fmt.Sprintf("Ошибка удаления запроса логов %s/%s: %s", counterID, requestID, err))
	}
	result, _ := jsonText(logReq)
	return result
}

func (s *Server) toolDownloadLog(ctx context.Context, args map[string]any) ToolCallResult {
	counterID := getString(args, "counter_id")
	requestID := getString(args, "request_id")
	if counterID == "" || requestID == "" {
		return errorContent("параметры counter_id и request_id обязательны")
	}
	partNumber := getIntString(args, "part_number", "0")

	data, err := s.metrika.DownloadLog(ctx, counterID, requestID, partNumber)
	if err != nil {
		return errorContent("Ошибка загрузки лога: " + err.Error())
	}
	// Logs are TSV text, return as-is.
	return textContent(data)
}
