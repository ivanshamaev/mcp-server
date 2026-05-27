package metrika

import (
	"context"
	"fmt"
	"net/url"
)

// Counter represents a Yandex Metrika counter.
type Counter struct {
	ID         int    `json:"id"`
	Status     string `json:"status"`
	OwnerLogin string `json:"owner_login"`
	Name       string `json:"name"`
	Site2      *Site  `json:"site2,omitempty"`
	Type       string `json:"type"`
}

// Site holds the site info of a counter.
type Site struct {
	Href string `json:"href"`
}

type countersResponse struct {
	Counters []Counter `json:"counters"`
}

type counterResponse struct {
	Counter Counter `json:"counter"`
}

// GetCounters returns the list of all counters available to the OAuth token.
func (c *Client) GetCounters(ctx context.Context) ([]Counter, error) {
	var resp countersResponse
	q := url.Values{"per_page": {"1000"}}
	if err := c.get(ctx, "/management/v1/counters", q, &resp); err != nil {
		return nil, fmt.Errorf("GetCounters: %w", err)
	}
	return resp.Counters, nil
}

// GetCounter returns details of a single counter.
func (c *Client) GetCounter(ctx context.Context, counterID string) (*Counter, error) {
	var resp counterResponse
	if err := c.get(ctx, "/management/v1/counter/"+counterID, nil, &resp); err != nil {
		return nil, fmt.Errorf("GetCounter %s: %w", counterID, err)
	}
	return &resp.Counter, nil
}
