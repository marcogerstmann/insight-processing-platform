package ingest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/mgerstmannsf/insight-processing-platform/internal/adapters/inbound/lambda/ingest/dto"
	"github.com/mgerstmannsf/insight-processing-platform/internal/application/apperr"
	"github.com/mgerstmannsf/insight-processing-platform/internal/application/ingestapp"
	"github.com/mgerstmannsf/insight-processing-platform/internal/application/tenant"
)

type Handler struct {
	Log    *slog.Logger
	Tenant *tenant.Resolver
	Ingest *ingestapp.Service
}

func NewHandler(log *slog.Logger, tr *tenant.Resolver, ingest *ingestapp.Service) *Handler {
	return &Handler{
		Log:    log,
		Tenant: tr,
		Ingest: ingest,
	}
}

func (h *Handler) Handle(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	receivedAt := time.Now().UTC()

	if req.Body == "" {
		// Readwise webhook verification request
		return jsonResponse(http.StatusOK, map[string]any{"status": "ok"}), nil
	}

	bodyBytes, err := readBody(req)
	if err != nil {
		h.Log.WarnContext(ctx, "invalid request body", "err", err)
		return jsonResponse(http.StatusBadRequest, map[string]any{
			"error": "invalid_request_body",
		}), nil
	}

	var payload dto.ReadwiseWebhookDTO
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		h.Log.WarnContext(ctx, "failed to parse json", "err", err)
		return jsonResponse(http.StatusBadRequest, map[string]any{"error": "invalid_json"}), nil
	}

	tenantCtx, err := h.Tenant.ResolveFromReadwise(payload)
	if err != nil {
		switch {
		case errors.Is(err, apperr.ErrUnauthorized):
			h.Log.WarnContext(ctx, "unauthorized_webhook",
				"event_type", payload.EventType,
				"highlight_id", payload.ID,
				"err", err,
			)
			return jsonResponse(http.StatusUnauthorized, map[string]any{"error": "unauthorized"}), nil

		case errors.Is(err, apperr.ErrServerMisconfigured):
			h.Log.ErrorContext(ctx, "server misconfigured", "err", err)
			return jsonResponse(http.StatusInternalServerError, map[string]any{"error": "server_misconfigured"}), nil

		default:
			h.Log.ErrorContext(ctx, "tenant resolution failed", "err", err)
			return jsonResponse(http.StatusInternalServerError, map[string]any{"error": "server_error"}), nil
		}
	}

	if err := h.Ingest.EnqueueReadwise(ctx, payload, receivedAt, tenantCtx.TenantID); err != nil {
		h.Log.ErrorContext(ctx, "enqueue failed", "err", err, "tenant_id", tenantCtx.TenantID)
		return jsonResponse(http.StatusInternalServerError, map[string]any{"error": "enqueue_failed"}), nil
	}

	h.Log.InfoContext(ctx, "readwise ingestion enqueued",
		"tenant_id", tenantCtx.TenantID,
		"event_type", payload.EventType,
		"highlight_id", payload.ID,
	)

	return jsonResponse(http.StatusOK, map[string]any{"status": "ok"}), nil
}

func readBody(req events.APIGatewayV2HTTPRequest) ([]byte, error) {
	if req.Body == "" {
		return nil, errors.New("empty body")
	}

	if !req.IsBase64Encoded {
		return []byte(req.Body), nil
	}

	return base64.StdEncoding.DecodeString(req.Body)
}

func jsonResponse(status int, payload any) events.APIGatewayV2HTTPResponse {
	b, _ := json.Marshal(payload)
	return events.APIGatewayV2HTTPResponse{
		StatusCode: status,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(b),
	}
}
