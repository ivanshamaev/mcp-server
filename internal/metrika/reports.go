package metrika

import (
	"context"
	"fmt"
	"net/url"
)

// ReportResponse is the raw response from the Reports API.
// We keep it as a generic map so all fields are available to the LLM.
type ReportResponse map[string]any

// GetReport fetches a statistics report from the Yandex Metrika Reports API.
// params is passed as-is as URL query parameters (id, metrics, dimensions, date1, date2, etc.)
func (c *Client) GetReport(ctx context.Context, params map[string]string) (ReportResponse, error) {
	q := make(url.Values)
	for k, v := range params {
		if v != "" {
			q.Set(k, v)
		}
	}

	var resp ReportResponse
	if err := c.get(ctx, "/stat/v1/data", q, &resp); err != nil {
		return nil, fmt.Errorf("GetReport: %w", err)
	}
	return resp, nil
}
