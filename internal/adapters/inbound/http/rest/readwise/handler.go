package readwise

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/http/rest/auth"
	readwiseclient "github.com/marcogerstmann/insight-processing-platform/internal/adapters/outbound/readwise"
	"github.com/marcogerstmann/insight-processing-platform/internal/application/ingest"
	"github.com/marcogerstmann/insight-processing-platform/internal/envutil"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

// readwiseTokenEnv is the env var (or "ssm:" reference, see envutil.ResolveSecret)
// holding the token used when a request doesn't supply its own.
const readwiseTokenEnv = "READWISE_API_TOKEN"

// readwiseEventType must match the Readwise webhook's event_type for created
// highlights (see apigw/readwise's webhookDTO and dev/http/readwise-webhook.http)
// so that a highlight imported here and the same highlight delivered via the
// webhook hash to the same idempotency key and dedupe against each other.
const readwiseEventType = "readwise.highlight.created"

type Handler struct {
	svc     ingest.Service
	secrets ports.SecretProvider
}

func NewHandler(svc ingest.Service, secrets ports.SecretProvider) *Handler {
	return &Handler{svc: svc, secrets: secrets}
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
		resolved, err := envutil.ResolveSecret(c.Request.Context(), readwiseTokenEnv, h.secrets)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "failed to resolve readwise token", "tenant_id", tenantID, "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_server_error"})
			return
		}
		token = resolved
	}
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "readwise token required: pass \"token\" or configure READWISE_API_TOKEN"})
		return
	}

	// Constructed per request (rather than once at startup) because token may
	// be a caller-supplied override rather than the server-configured one.
	importer := ingest.NewImporter(readwiseclient.NewClient(token), h.svc, "readwise", readwiseEventType)
	result, err := importer.Import(c.Request.Context(), tenantID, req.Limit, req.OnlyFavorites)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "readwise import failed", "tenant_id", tenantID, "err", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "readwise_import_failed"})
		return
	}

	c.JSON(http.StatusOK, ImportResponseDTO{Fetched: result.Fetched, Enqueued: result.Enqueued})
}
