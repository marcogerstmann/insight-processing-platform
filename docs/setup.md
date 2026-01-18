# Setup (minimal)

## Environment variables

| Variable                | Description                                   |
|-------------------------|-----------------------------------------------|
| READWISE_WEBHOOK_SECRET | Verifies incoming Readwise webhook signatures |

## Local ingest simulation

```bash
go run ./cmd/ingest-local/main.go
```

Prepared test payloads live in `./httpRequests.`