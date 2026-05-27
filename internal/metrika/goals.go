package metrika

import (
	"context"
	"fmt"
)

// Goal represents a Yandex Metrika goal.
type Goal struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Flag       string `json:"flag"`
	Conditions []any  `json:"conditions,omitempty"`
}

type goalsResponse struct {
	Goals []Goal `json:"goals"`
}

// GetGoals returns the list of goals configured for a counter.
func (c *Client) GetGoals(ctx context.Context, counterID string) ([]Goal, error) {
	var resp goalsResponse
	if err := c.get(ctx, "/management/v1/counter/"+counterID+"/goals", nil, &resp); err != nil {
		return nil, fmt.Errorf("GetGoals %s: %w", counterID, err)
	}
	return resp.Goals, nil
}

// Segment represents a Yandex Metrika segment.
type Segment struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Expression string `json:"expression"`
	Status     string `json:"status"`
}

type segmentsResponse struct {
	Segments []Segment `json:"segments"`
}

// GetSegments returns the list of segments for a counter.
func (c *Client) GetSegments(ctx context.Context, counterID string) ([]Segment, error) {
	var resp segmentsResponse
	if err := c.get(ctx, "/management/v1/counter/"+counterID+"/segments", nil, &resp); err != nil {
		return nil, fmt.Errorf("GetSegments %s: %w", counterID, err)
	}
	return resp.Segments, nil
}
