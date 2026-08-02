package readwise

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

// readwiseTokenEnv is the env var (or "ssm:" reference, see envutil.ResolveSecret)
// holding the token used when a request doesn't supply its own.
const readwiseTokenEnv = "READWISE_API_TOKEN"

type Handler struct {
	importer *ingest.Importer
	secrets  ports.SecretProvider
}

func NewHandler(importer *ingest.Importer, secrets ports.SecretProvider) *Handler {
	return &Handler{importer: importer, secrets: secrets}
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

	result, err := h.importer.Import(c.Request.Context(), tenantID, token, req.Limit, req.OnlyFavorites)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "readwise import failed", "tenant_id", tenantID, "err", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "readwise_import_failed"})
		return
	}

	c.JSON(http.StatusOK, ImportResponseDTO{Fetched: result.Fetched, Enqueued: result.Enqueued})
}
