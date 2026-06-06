package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/joho/godotenv"

	"github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/apigw/readwise"
	"github.com/marcogerstmann/insight-processing-platform/internal/adapters/outbound/sqs"
	"github.com/marcogerstmann/insight-processing-platform/internal/adapters/outbound/ssm"
	"github.com/marcogerstmann/insight-processing-platform/internal/application/ingest"
	"github.com/marcogerstmann/insight-processing-platform/internal/application/tenant"
	"github.com/marcogerstmann/insight-processing-platform/internal/logging"
)

func main() {
	_ = godotenv.Load()

	log := logging.New(os.Stdout)
	slog.SetDefault(log)

	ctx := context.Background()

	publisher, err := sqs.NewSQSEventPublisher(ctx)
	if err != nil {
		log.Error("publisher init failed", "err", err)
		os.Exit(1)
	}

	secretProvider, err := ssm.NewSecretProvider(ctx)
	if err != nil {
		log.Error("ssm provider init failed", "err", err)
		os.Exit(1)
	}

	ingestSvc := ingest.NewService(publisher)
	tenantResolver := tenant.NewResolver()

	handler := readwise.NewHandler(secretProvider, tenantResolver, ingestSvc)

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
			slog.Error("handler error", "err", err)
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
	slog.Info("listening", "addr", "http://localhost"+addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
