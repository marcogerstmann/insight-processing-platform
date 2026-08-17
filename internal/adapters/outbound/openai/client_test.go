package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sdk "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// testClient points the SDK at a local server. Same shape as the readwise
// adapter's test: construct the struct directly rather than adding a
// production seam that only tests use.
func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	return &Client{
		client: sdk.NewClient(
			option.WithBaseURL(srv.URL),
			option.WithAPIKey("test-key"),
			option.WithMaxRetries(0),
		),
		model:     enrichModel,
		maxTokens: defaultMaxTokens,
		timeout:   5 * time.Second,
	}
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func completionJSON(content string) map[string]any {
	return map[string]any{
		"id":      "chatcmpl-test",
		"created": 1,
		"model":   enrichModel,
		"object":  "chat.completion",
		"choices": []map[string]any{{
			"index":         0,
			"finish_reason": "stop",
			"message":       map[string]any{"role": "assistant", "content": content},
		}},
		"usage": map[string]any{
			"prompt_tokens": 42, "completion_tokens": 7, "total_tokens": 49,
		},
	}
}

func TestEnrich_RequestsAStrictSchemaAndReturnsTheTags(t *testing.T) {
	var body map[string]any

	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("unexpected auth header: %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeJSON(w, completionJSON(`{"tags":["psychology","habit-formation"]}`))
	})

	got, err := c.Enrich(context.Background(), "a highlight about habits")
	if err != nil {
		t.Fatalf("Enrich returned error: %v", err)
	}

	if len(got.Tags) != 2 || got.Tags[0] != "psychology" || got.Tags[1] != "habit-formation" {
		t.Fatalf("unexpected tags: %v", got.Tags)
	}
	if body["model"] != enrichModel {
		t.Errorf("expected model %q, got %v", enrichModel, body["model"])
	}
	// strict is what makes the unmarshal below safe to do without a
	// fallback path — if it stops being sent, the parse can start failing
	// on prose instead of JSON.
	format, _ := body["response_format"].(map[string]any)
	schema, _ := format["json_schema"].(map[string]any)
	if schema["strict"] != true {
		t.Errorf("expected strict schema, got response_format %v", format)
	}
	if body["max_completion_tokens"] != float64(defaultMaxTokens) {
		t.Errorf("expected a bounded token ceiling, got %v", body["max_completion_tokens"])
	}
}

func TestEnrich_ErrorsWhenTheResponseHasNoChoices(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		empty := completionJSON("")
		empty["choices"] = []map[string]any{}
		writeJSON(w, empty)
	})

	if _, err := c.Enrich(context.Background(), "text"); err == nil {
		t.Fatal("expected an error when the response carries no choices")
	}
}

func TestEnrich_ErrorsWhenTheContentIsNotTheSchema(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, completionJSON("I'm afraid I can't do that."))
	})

	if _, err := c.Enrich(context.Background(), "text"); err == nil {
		t.Fatal("expected an error when the content is not valid schema JSON")
	}
}
