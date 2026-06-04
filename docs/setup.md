# Setup (minimal)

## Environment variables

| Variable                | Description                                                                                                                              |
|-------------------------|------------------------------------------------------------------------------------------------------------------------------------------|
| DEFAULT_TENANT_ID       | Default tenant ID.                                                                                                                       |
| TABLE_NAME_INSIGHTS     | Name of the insights table in DynamoDB.                                                                                                  |
| INGEST_QUEUE_URL        | URL for the ingestion queue.                                                                                                             |
| INGEST_DLQ_URL          | URL for the ingestion DLQ.                                                                                                               |
| READWISE_WEBHOOK_SECRET | Verifies incoming Readwise webhook signatures.                                                                                           |
| ANTHROPIC_API_KEY       | Optional. Anthropic API key (local dev). If unset, falls back to `ANTHROPIC_API_KEY_SSM`. If neither is set, LLM enrichment is disabled. |
| ANTHROPIC_API_KEY_SSM   | Optional. SSM parameter path for the Anthropic API key (AWS cloud).                                                                      |

## Local ingest simulation

```bash
go run ./cmd/ingest-local/main.go
```

Prepared test payloads live in `./dev/http`
