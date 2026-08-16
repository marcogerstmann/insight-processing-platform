package readwise

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchHighlights_PaginatesFiltersAndSortsNewestFirst(t *testing.T) {
	pages := map[string]exportResponse{
		"": {
			NextPageCursor: "page2",
			Results: []exportBook{{
				Highlights: []exportHighlight{
					{ID: 1, Text: "older", UpdatedAt: mustParse("2026-01-01T00:00:00Z")},
					{ID: 2, Text: "gone", IsDeleted: true, UpdatedAt: mustParse("2026-06-01T00:00:00Z")},
				},
			}},
		},
		"page2": {
			NextPageCursor: "",
			Results: []exportBook{{
				Highlights: []exportHighlight{
					{ID: 3, Text: "newer", UpdatedAt: mustParse("2026-03-01T00:00:00Z")},
				},
			}},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Token test-token" {
			t.Fatalf("unexpected auth header: %q", got)
		}
		cursor := r.URL.Query().Get("pageCursor")
		page, ok := pages[cursor]
		if !ok {
			t.Fatalf("unexpected pageCursor: %q", cursor)
		}
		_ = json.NewEncoder(w).Encode(page)
	}))
	defer srv.Close()

	c := &Client{httpClient: srv.Client(), baseURL: srv.URL, token: "test-token"}

	got, err := c.FetchHighlights(context.Background())
	if err != nil {
		t.Fatalf("FetchHighlights returned error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 non-deleted highlights, got %d", len(got))
	}
	if got[0].ID != "3" || got[1].ID != "1" {
		t.Fatalf("expected newest-first order [3,1], got [%s,%s]", got[0].ID, got[1].ID)
	}
}

func TestFetchHighlights_NumericNextPageCursor(t *testing.T) {
	// Readwise's docs describe nextPageCursor as a string, but it has been
	// observed on the wire as a bare JSON number. Both must decode cleanly.
	responses := map[string]string{
		"": `{"nextPageCursor": 12345, "results": [{"highlights": [
			{"id": 1, "text": "first", "updated_at": "2026-01-01T00:00:00Z"}
		]}]}`,
		"12345": `{"nextPageCursor": null, "results": [{"highlights": [
			{"id": 2, "text": "second", "updated_at": "2026-02-01T00:00:00Z"}
		]}]}`,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := responses[r.URL.Query().Get("pageCursor")]
		if !ok {
			t.Fatalf("unexpected pageCursor: %q", r.URL.Query().Get("pageCursor"))
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := &Client{httpClient: srv.Client(), baseURL: srv.URL, token: "test-token"}

	got, err := c.FetchHighlights(context.Background())
	if err != nil {
		t.Fatalf("FetchHighlights returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 highlights across both pages, got %d", len(got))
	}
}

func TestFetchHighlights_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := &Client{httpClient: srv.Client(), baseURL: srv.URL, token: "bad-token"}

	_, err := c.FetchHighlights(context.Background())
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func mustParse(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}
