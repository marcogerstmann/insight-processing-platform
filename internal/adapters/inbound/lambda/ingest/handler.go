package ingest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
)

var log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

type TenantContext struct {
	TenantID string
}

var publisher *SQSPublisher

func Handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	receivedAt := time.Now().UTC()

	if req.Body == "" {
		// This is likely the Readwise webhook verification request
		return jsonResponse(http.StatusOK, map[string]any{
			"status": "ok",
		}), nil
	}

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

	tenantCtx, err := resolveTenant(payload)
	if err != nil {
		switch err.Error() {
		case "unauthorized":
			log.WarnContext(ctx, "unauthorized_webhook",
				"event_type", payload.EventType,
				"highlight_id", payload.ID,
			)
			return jsonResponse(http.StatusUnauthorized, map[string]any{"error": "unauthorized"}), nil
		case "server_misconfigured":
			log.ErrorContext(ctx, "server misconfigured", "err", err)
			return jsonResponse(http.StatusInternalServerError, map[string]any{"error": "server_misconfigured"}), nil
		default:
			log.ErrorContext(ctx, "tenant resolution failed", "err", err)
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

	idempotencyKey := BuildIdempotencyKey(ev)

	if publisher == nil {
		p, err := NewSQSPublisher(ctx)
		if err != nil {
			log.ErrorContext(ctx, "sqs_publisher_init_failed", "err", err)
			return jsonResponse(http.StatusInternalServerError, map[string]any{
				"error": "server_misconfigured",
			}), nil
		}
		publisher = p
	}

	if err := publisher.Publish(ctx, ev, idempotencyKey, receivedAt); err != nil {
		log.ErrorContext(ctx, "sqs publish failed",
			"err", err,
			"tenant_id", ev.TenantID,
			"idempotency_key", idempotencyKey,
			"event_type", ev.EventType,
			"highlight_id", ev.Highlight.ID,
		)
		return jsonResponse(http.StatusInternalServerError, map[string]any{
			"error": "enqueue_failed",
		}), nil
	}

	log.InfoContext(ctx, "readwise ingestion enqueued",
		"tenant_id", ev.TenantID,
		"idempotency_key", idempotencyKey,
		"event_type", ev.EventType,
		"highlight_id", ev.Highlight.ID,
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
