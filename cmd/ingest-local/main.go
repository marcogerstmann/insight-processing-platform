package main

import (
	"context"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/joho/godotenv"

	ingestlambda "github.com/mgerstmannsf/insight-processing-platform/internal/adapters/inbound/lambda/ingest"
	"github.com/mgerstmannsf/insight-processing-platform/internal/adapters/outbound/sqs"
	"github.com/mgerstmannsf/insight-processing-platform/internal/application/ingest"
	"github.com/mgerstmannsf/insight-processing-platform/internal/application/tenant"
)

func main() {
	_ = godotenv.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	ctx := context.Background()

	publisher, err := sqs.NewSQSEventPublisher(ctx)
	if err != nil {
		log.Fatalf("publisher init failed: %v", err)
	}

	ingestSvc := ingest.NewService(publisher)
	tenantResolver := tenant.NewResolver( /* config */ )

	handler := ingestlambda.NewHandler(logger, tenantResolver, ingestSvc)

	mux := http.NewServeMux()
	mux.HandleFunc("/readwise/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		req := events.APIGatewayV2HTTPRequest{
			Version:         "2.0",
			RouteKey:        "POST /readwise/webhook",
			RawPath:         r.URL.Path,
			RawQueryString:  r.URL.RawQuery,
			Headers:         map[string]string{},
			Body:            string(body),
			IsBase64Encoded: false,
		}

		for k, v := range r.Header {
			if len(v) > 0 {
				req.Headers[k] = v[0]
			}
		}

		resp, err := handler.Handle(ctx, req)
		if err != nil {
			logger.Error("handler error", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		for k, v := range resp.Headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write([]byte(resp.Body))
	})

	addr := ":8080"
	log.Printf("listening on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
