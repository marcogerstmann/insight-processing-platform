package ingest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
)

var log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

type TenantContext struct {
	TenantID string
}

func Handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	receivedAt := time.Now().UTC()

	bodyBytes, err := readBody(req)
	if err != nil {
		log.WarnContext(ctx, "failed to read body", "err", err)
		return jsonResponse(http.StatusBadRequest, map[string]any{
			"error": "invalid_request_body",
		}), nil
	}

	var payload ReadwiseWebhookDTO
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		log.WarnContext(ctx, "failed to parse json", "err", err)
		return jsonResponse(http.StatusBadRequest, map[string]any{
			"error": "invalid_json",
		}), nil
	}

	tenantCtx, err := resolveTenant(ctx, payload)
	if err != nil {
		switch err.Error() {
		case "unauthorized":
			log.WarnContext(ctx, "unauthorized_webhook",
				"event_type", payload.EventType,
				"highlight_id", payload.ID,
			)
			return jsonResponse(http.StatusUnauthorized, map[string]any{"error": "unauthorized"}), nil
		case "server_misconfigured":
			return jsonResponse(http.StatusInternalServerError, map[string]any{"error": "server_misconfigured"}), nil
		default:
			log.ErrorContext(ctx, "tenant_resolution_failed", "err", err)
			return jsonResponse(http.StatusInternalServerError, map[string]any{"error": "server_error"}), nil
		}
	}

	ev, err := MapReadwisePayload(payload, receivedAt)
	if err != nil {
		status := http.StatusBadRequest
		if !errors.Is(err, ErrInvalidPayload) {
			status = http.StatusInternalServerError
		}
		log.WarnContext(ctx, "mapping_failed", "err", err)
		return jsonResponse(status, map[string]any{
			"error": "invalid_payload",
		}), nil
	}

	ev.TenantID = tenantCtx.TenantID

	// TODO: For now log the normalized event. Later this becomes "publish to SQS"
	log.InfoContext(ctx, "readwise ingestion ok",
		"tenant_id", ev.TenantID,
		"source", ev.Source,
		"event_type", ev.EventType,
		"highlight_id", ev.Highlight.ID,
		"book_id", ev.Highlight.BookID,
		"url", ev.Highlight.URL,
		"highlighted_at", ev.Highlight.HighlightedAt,
		"received_at", ev.ReceivedAt,
	)

	return jsonResponse(http.StatusOK, map[string]any{
		"status": "ok",
	}), nil
}

func readBody(req events.APIGatewayV2HTTPRequest) ([]byte, error) {
	if req.Body == "" {
		return nil, errors.New("empty body")
	}
	if !req.IsBase64Encoded {
		return []byte(req.Body), nil
	}
	decoded, err := base64.StdEncoding.DecodeString(req.Body)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func resolveTenant(ctx context.Context, payload ReadwiseWebhookDTO) (TenantContext, error) {
	// Today: single-tenant mode, using env secret + fixed tenant id.
	// Later: extract secret from header/query, look up tenant in DB, verify signature, etc.
	tenantID := strings.TrimSpace(os.Getenv("DEFAULT_TENANT_ID"))
	if tenantID == "" {
		log.ErrorContext(ctx, "missing environment variable", "env", "DEFAULT_TENANT_ID")
		return TenantContext{}, errors.New("server_misconfigured")
	}

	secretExpected := strings.TrimSpace(os.Getenv("READWISE_WEBHOOK_SECRET"))
	if secretExpected == "" {
		log.ErrorContext(ctx, "missing environment variable", "env", "READWISE_WEBHOOK_SECRET")
		return TenantContext{}, errors.New("server_misconfigured")
	}

	if strings.TrimSpace(payload.Secret) != secretExpected {
		return TenantContext{}, errors.New("unauthorized")
	}

	return TenantContext{TenantID: tenantID}, nil
}

func jsonResponse(status int, payload any) events.APIGatewayV2HTTPResponse {
	b, _ := json.Marshal(payload) // safe for small known payloads; ignore marshal error
	return events.APIGatewayV2HTTPResponse{
		StatusCode: status,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(b),
	}
}
