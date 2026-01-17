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

	secretExpected := strings.TrimSpace(os.Getenv("READWISE_WEBHOOK_SECRET"))
	if secretExpected == "" {
		log.ErrorContext(ctx, "missing environment variable", "env", "READWISE_WEBHOOK_SECRET")
		return jsonResponse(http.StatusInternalServerError, map[string]any{
			"error": "server_misconfigured",
		}), nil
	}
	if strings.TrimSpace(payload.Secret) != secretExpected {
		log.WarnContext(ctx, "secrets not matching",
			"event_type", payload.EventType,
			"highlight_id", payload.ID,
		)
		return jsonResponse(http.StatusUnauthorized, map[string]any{
			"error": "unauthorized",
		}), nil
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

	// TODO: For now log the normalized event. Later this becomes "publish to SQS".
	log.InfoContext(ctx, "readwise ingestion ok",
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
