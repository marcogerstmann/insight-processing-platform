package raindrop

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/http/rest/auth"
	"github.com/marcogerstmann/insight-processing-platform/internal/application/ingest"
	"github.com/marcogerstmann/insight-processing-platform/internal/envutil"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

// raindropTokenEnv is the env var (or "ssm:" reference, see envutil.ResolveSecret)
// holding the token used when a request doesn't supply its own.
const raindropTokenEnv = "RAINDROP_API_TOKEN"

// raindropEventType is stamped on every highlight imported from Raindrop.
// Raindrop has no push webhook (unlike Readwise), so unlike
// readwiseEventType this doesn't need to match anything else — it only
// needs to be stable across imports and poll runs so re-fetched highlights
// dedupe against each other via buildIdempotencyKey.
const raindropEventType = "raindrop.highlight.created"

type Handler struct {
	svc     ingest.Service
	secrets ports.SecretProvider
	// newSource builds the HighlightSource for a resolved token. Supplied by
	// the composition root (cmd/) rather than constructed here, so this
	// package never imports the concrete raindrop client adapter — and
	// tests can swap in a fake without making a real HTTP request.
	newSource func(token string) ports.HighlightSource
}

func NewHandler(svc ingest.Service, secrets ports.SecretProvider, newSource func(token string) ports.HighlightSource) *Handler {
	return &Handler{svc: svc, secrets: secrets, newSource: newSource}
}

func (h *Handler) Import(c *gin.Context) {
	tenantID := c.GetString(auth.TenantIDKey)

	var req ImportRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request_body"})
		return
	}

	token := strings.TrimSpace(req.Token)
	if token == "" {
		resolved, err := envutil.ResolveSecret(c.Request.Context(), raindropTokenEnv, h.secrets)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "failed to resolve raindrop token", "tenant_id", tenantID, "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_server_error"})
			return
		}
		token = resolved
	}
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "raindrop token required: pass \"token\" or configure RAINDROP_API_TOKEN"})
		return
	}

	// Constructed per request (rather than once at startup) because token may
	// be a caller-supplied override rather than the server-configured one.
	importer := ingest.NewImporter(h.newSource(token), h.svc, "raindrop", raindropEventType)
	result, err := importer.Import(c.Request.Context(), tenantID, req.Limit, false)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "raindrop import failed", "tenant_id", tenantID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "raindrop_import_failed"})
		return
	}

	c.JSON(http.StatusOK, ImportResponseDTO{Fetched: result.Fetched, Enqueued: result.Enqueued})
}
