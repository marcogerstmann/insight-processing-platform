# Setup (minimal)

## Environment variables

| Variable                | Description                                   |
|-------------------------|-----------------------------------------------|
| READWISE_WEBHOOK_SECRET | Verifies incoming Readwise webhook signatures |
| INGEST_QUEUE_URL        | URL for the ingestion queue                   |
| DEFAULT_TENANT_ID       | Default tenant ID                             |
| TABLE_NAME_INSIGHTS     | Name of the insights table in DynamoDB        |

## Local ingest simulation

```bash
go run ./cmd/ingest-local/main.go
```

Prepared test payloads live in `./dev/http`
