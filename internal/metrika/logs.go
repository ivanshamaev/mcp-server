package metrika

import (
	"context"
	"fmt"
	"net/url"
)

// LogRequest represents a Yandex Metrika Logs API request.
type LogRequest struct {
	RequestID int    `json:"request_id"`
	CounterID int    `json:"counter_id"`
	Source    string `json:"source"`
	Date1     string `json:"date1"`
	Date2     string `json:"date2"`
	Fields    string `json:"fields"`
	Status    string `json:"status"`
	Size      int    `json:"size,omitempty"`
	Parts     []any  `json:"parts,omitempty"`
}

type logRequestsResponse struct {
	Requests []LogRequest `json:"requests"`
}

type logRequestResponse struct {
	LogRequest LogRequest `json:"log_request"`
}

// ListLogs returns all log requests for a counter.
func (c *Client) ListLogs(ctx context.Context, counterID string) ([]LogRequest, error) {
	var resp logRequestsResponse
	if err := c.get(ctx, "/logs/v1/counter/"+counterID+"/logrequests", nil, &resp); err != nil {
		return nil, fmt.Errorf("ListLogs %s: %w", counterID, err)
	}
	return resp.Requests, nil
}

// CreateLogRequest creates a new log download request.
func (c *Client) CreateLogRequest(ctx context.Context, counterID, fields, source, date1, date2 string) (*LogRequest, error) {
	q := url.Values{
		"fields": {fields},
		"source": {source},
		"date1":  {date1},
		"date2":  {date2},
	}
	var resp logRequestResponse
	if err := c.post(ctx, "/logs/v1/counter/"+counterID+"/logrequests", q, &resp); err != nil {
		return nil, fmt.Errorf("CreateLogRequest %s: %w", counterID, err)
	}
	return &resp.LogRequest, nil
}

// GetLogRequest returns the status of a specific log request.
func (c *Client) GetLogRequest(ctx context.Context, counterID, requestID string) (*LogRequest, error) {
	var resp logRequestResponse
	path := fmt.Sprintf("/logs/v1/counter/%s/logrequest/%s", counterID, requestID)
	if err := c.get(ctx, path, nil, &resp); err != nil {
		return nil, fmt.Errorf("GetLogRequest %s/%s: %w", counterID, requestID, err)
	}
	return &resp.LogRequest, nil
}

// CleanLogRequest deletes a completed log request, freeing the slot.
// Metrika limits the number of concurrent log requests per counter.
func (c *Client) CleanLogRequest(ctx context.Context, counterID, requestID string) (*LogRequest, error) {
	var resp logRequestResponse
	path := fmt.Sprintf("/logs/v1/counter/%s/logrequest/%s/clean", counterID, requestID)
	if err := c.post(ctx, path, nil, &resp); err != nil {
		return nil, fmt.Errorf("CleanLogRequest %s/%s: %w", counterID, requestID, err)
	}
	return &resp.LogRequest, nil
}

// DownloadLog downloads a specific part of a completed log request.
// Returns raw TSV text.
func (c *Client) DownloadLog(ctx context.Context, counterID, requestID, partNumber string) (string, error) {
	path := fmt.Sprintf("/logs/v1/counter/%s/logrequest/%s/part/%s/download",
		counterID, requestID, partNumber)
	data, err := c.getRaw(ctx, path, nil)
	if err != nil {
		return "", fmt.Errorf("DownloadLog %s/%s part%s: %w", counterID, requestID, partNumber, err)
	}
	return data, nil
}
