package metrika

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetCounters(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "OAuth test-token" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(countersResponse{
			Counters: []Counter{
				{ID: 1, Name: "Test Counter", Status: "Active"},
				{ID: 2, Name: "Another Counter", Status: "Active"},
			},
		})
	}))
	defer ts.Close()

	client := NewClient("test-token", WithBaseURL(ts.URL))
	counters, err := client.GetCounters(context.Background())
	if err != nil {
		t.Fatalf("GetCounters error: %v", err)
	}
	if len(counters) != 2 {
		t.Errorf("want 2 counters, got %d", len(counters))
	}
	if counters[0].Name != "Test Counter" {
		t.Errorf("want 'Test Counter', got %s", counters[0].Name)
	}
}

func TestGetCounter(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/management/v1/counter/12345" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(counterResponse{
			Counter: Counter{ID: 12345, Name: "My Site"},
		})
	}))
	defer ts.Close()

	client := NewClient("tok", WithBaseURL(ts.URL))
	counter, err := client.GetCounter(context.Background(), "12345")
	if err != nil {
		t.Fatalf("GetCounter error: %v", err)
	}
	if counter.ID != 12345 {
		t.Errorf("want ID 12345, got %d", counter.ID)
	}
}

func TestGetCounters_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"errors":[{"error_type":"not_found"}]}`, http.StatusNotFound)
	}))
	defer ts.Close()

	client := NewClient("tok", WithBaseURL(ts.URL))
	_, err := client.GetCounters(context.Background())
	if err == nil {
		t.Fatal("expected error for HTTP 404")
	}
}

func TestGetReport(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stat/v1/data" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("id") == "" {
			t.Error("expected 'id' query param")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data":       []any{},
			"total_rows": 0,
		})
	}))
	defer ts.Close()

	client := NewClient("tok", WithBaseURL(ts.URL))
	report, err := client.GetReport(context.Background(), map[string]string{
		"id":      "12345",
		"metrics": "ym:s:visits",
	})
	if err != nil {
		t.Fatalf("GetReport error: %v", err)
	}
	if report == nil {
		t.Error("expected non-nil report")
	}
}
