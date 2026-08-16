package raindrop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/http/rest/auth"
	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type fakeHighlightSource struct {
	highlights []ports.SourceHighlight
	err        error
}

func (f *fakeHighlightSource) FetchHighlights(_ context.Context) ([]ports.SourceHighlight, error) {
	return f.highlights, f.err
}

type fakeService struct{ enqueued int }

func (f *fakeService) Enqueue(_ context.Context, _ domain.IngestEvent) error {
	f.enqueued++
	return nil
}

type fakeSecretProvider struct{}

func (fakeSecretProvider) Get(_ context.Context, _ string) (string, error) { return "", nil }

func newTestHandler(svc *fakeService, source *fakeHighlightSource) *Handler {
	return &Handler{
		svc:       svc,
		secrets:   fakeSecretProvider{},
		newSource: func(string) ports.HighlightSource { return source },
	}
}

func doImport(h *Handler, tenantID string, body []byte) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/raindrop/import", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(auth.TenantIDKey, tenantID)

	h.Import(c)
	return w
}

func TestImport_Success(t *testing.T) {
	svc := &fakeService{}
	source := &fakeHighlightSource{highlights: []ports.SourceHighlight{
		{ID: "1", Text: "first"},
		{ID: "2", Text: "second"},
	}}
	h := newTestHandler(svc, source)

	w := doImport(h, "tenant-1", []byte(`{"token":"explicit-token"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp ImportResponseDTO
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Fetched != 2 || resp.Enqueued != 2 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if svc.enqueued != 2 {
		t.Fatalf("expected 2 enqueued, got %d", svc.enqueued)
	}
}

func TestImport_FallsBackToEnvToken(t *testing.T) {
	t.Setenv("RAINDROP_API_TOKEN", "env-token")
	svc := &fakeService{}
	source := &fakeHighlightSource{highlights: []ports.SourceHighlight{{ID: "1", Text: "first"}}}
	h := newTestHandler(svc, source)

	// No "token" field in the body — must resolve RAINDROP_API_TOKEN instead.
	w := doImport(h, "tenant-1", []byte(`{}`))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if svc.enqueued != 1 {
		t.Fatalf("expected 1 enqueued, got %d", svc.enqueued)
	}
}

func TestImport_MissingTokenReturns400(t *testing.T) {
	svc := &fakeService{}
	h := newTestHandler(svc, &fakeHighlightSource{})

	// Neither a body token nor RAINDROP_API_TOKEN is set.
	w := doImport(h, "tenant-1", nil)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if svc.enqueued != 0 {
		t.Fatalf("expected no highlights enqueued, got %d", svc.enqueued)
	}
}

func TestImport_UpstreamErrorReturns500(t *testing.T) {
	svc := &fakeService{}
	source := &fakeHighlightSource{err: errors.New("raindrop: invalid API token")}
	h := newTestHandler(svc, source)

	w := doImport(h, "tenant-1", []byte(`{"token":"bad-token"}`))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}
