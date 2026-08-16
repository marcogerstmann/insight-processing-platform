package main

import (
	"context"
	"log/slog"
	"os"
	"strconv"

	"github.com/joho/godotenv"

	scheduleraindrop "github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/schedule/raindrop"
	raindropclient "github.com/marcogerstmann/insight-processing-platform/internal/adapters/outbound/raindrop"
	"github.com/marcogerstmann/insight-processing-platform/internal/adapters/outbound/sqs"
	"github.com/marcogerstmann/insight-processing-platform/internal/adapters/outbound/ssm"
	"github.com/marcogerstmann/insight-processing-platform/internal/application/ingest"
	"github.com/marcogerstmann/insight-processing-platform/internal/application/tenant"
	"github.com/marcogerstmann/insight-processing-platform/internal/envutil"
	"github.com/marcogerstmann/insight-processing-platform/internal/logging"
)

// defaultPollLimit caps how many highlights a single run enqueues, so a poll
// never re-enqueues the entire corpus. Override via RAINDROP_POLL_LIMIT.
const defaultPollLimit = 50

// Runs a single Raindrop poll and exits — the local counterpart to
// raindrop-poll-lambda, for exercising the handler against real Raindrop and
// SQS without deploying. EventBridge Scheduler invokes the Lambda on a
// recurring cadence (RAINDROP 6); this runner does one pass and stops.
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

	token, err := envutil.ResolveSecret(ctx, "RAINDROP_API_TOKEN", secretProvider)
	if err != nil {
		log.Error("failed to resolve raindrop token", "err", err)
		os.Exit(1)
	}
	if token == "" {
		log.Error("RAINDROP_API_TOKEN is required")
		os.Exit(1)
	}

	tenantCtx, err := tenant.NewResolver().Resolve()
	if err != nil {
		log.Error("tenant resolution failed", "err", err)
		os.Exit(1)
	}

	ingestSvc := ingest.NewService(publisher)
	client := raindropclient.NewClient(token)
	h := scheduleraindrop.NewHandler(client, ingestSvc, tenantCtx.TenantID, pollLimit(log))

	if err := h.Poll(ctx); err != nil {
		log.Error("raindrop poll failed", "err", err)
		os.Exit(1)
	}
}

func pollLimit(log *slog.Logger) int {
	v := os.Getenv("RAINDROP_POLL_LIMIT")
	if v == "" {
		return defaultPollLimit
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		log.Warn("invalid RAINDROP_POLL_LIMIT, using default", "value", v, "default", defaultPollLimit)
		return defaultPollLimit
	}
	return n
}
