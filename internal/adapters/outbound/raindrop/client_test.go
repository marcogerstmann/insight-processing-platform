package raindrop

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchHighlights_PaginatesAndSortsNewestFirst(t *testing.T) {
	// perPage is 50, so a first page of exactly 50 items must trigger a
	// second request; the second page (1 item) being short ends pagination.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("unexpected auth header: %q", got)
		}
		page := r.URL.Query().Get("page")
		if got := r.URL.Query().Get("perpage"); got != "50" {
			t.Fatalf("unexpected perpage: %q", got)
		}

		var items []string
		switch page {
		case "0":
			for i := range perPage {
				items = append(items, fmt.Sprintf(
					`{"_id":"%d","text":"h%d","created":"2026-01-01T00:00:00.000Z"}`, i, i))
			}
		case "1":
			items = append(items, `{"_id":"newest","text":"newest","created":"2026-06-01T00:00:00.000Z"}`)
		default:
			t.Fatalf("unexpected page: %q", page)
		}

		_, _ = w.Write([]byte(fmt.Sprintf(`{"result":true,"items":[%s]}`, strings.Join(items, ","))))
	}))
	defer srv.Close()

	c := &Client{httpClient: srv.Client(), baseURL: srv.URL, token: "test-token"}

	got, err := c.FetchHighlights(context.Background())
	if err != nil {
		t.Fatalf("FetchHighlights returned error: %v", err)
	}
	if len(got) != perPage+1 {
		t.Fatalf("expected %d highlights across both pages, got %d", perPage+1, len(got))
	}
	if got[0].ID != "newest" {
		t.Fatalf("expected newest-first order, got id=%s first", got[0].ID)
	}
}

func TestFetchHighlights_EmptyResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":true,"items":[]}`))
	}))
	defer srv.Close()

	c := &Client{httpClient: srv.Client(), baseURL: srv.URL, token: "test-token"}

	got, err := c.FetchHighlights(context.Background())
	if err != nil {
		t.Fatalf("FetchHighlights returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 highlights, got %d", len(got))
	}
}

func TestFetchHighlights_FieldMappingWithEmptyNote(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":true,"items":[
			{"_id":"1","text":"hi","link":"https://example.com","created":"2026-03-21T14:41:34.059Z"}
		]}`))
	}))
	defer srv.Close()

	c := &Client{httpClient: srv.Client(), baseURL: srv.URL, token: "test-token"}

	got, err := c.FetchHighlights(context.Background())
	if err != nil {
		t.Fatalf("FetchHighlights returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 highlight, got %d", len(got))
	}
	h := got[0]
	if h.ID != "1" || h.Text != "hi" || h.Note != "" {
		t.Fatalf("unexpected mapping: %+v", h)
	}
	if h.URL == nil || *h.URL != "https://example.com" {
		t.Fatalf("expected URL to be mapped from link, got %v", h.URL)
	}
	if h.IsFavorite {
		t.Fatalf("expected IsFavorite=false, Raindrop has no favourites concept")
	}
	want := mustParse("2026-03-21T14:41:34.059Z")
	if !h.HighlightedAt.Equal(want) {
		t.Fatalf("HighlightedAt = %v, want %v", h.HighlightedAt, want)
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

func TestFetchHighlights_RateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := &Client{httpClient: srv.Client(), baseURL: srv.URL, token: "test-token"}

	_, err := c.FetchHighlights(context.Background())
	if err == nil || !strings.Contains(err.Error(), "30") {
		t.Fatalf("expected rate-limit error surfacing Retry-After, got: %v", err)
	}
}

func TestFetchHighlights_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{not json`))
	}))
	defer srv.Close()

	c := &Client{httpClient: srv.Client(), baseURL: srv.URL, token: "test-token"}

	_, err := c.FetchHighlights(context.Background())
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func mustParse(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}
