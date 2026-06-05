# Setup (minimal)

## Environment variables

| Variable                | Description                                                                                                                                            |
|-------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------|
| DEFAULT_TENANT_ID       | Default tenant ID.                                                                                                                                     |
| TABLE_NAME_INSIGHTS     | Name of the insights table in DynamoDB.                                                                                                                |
| INGEST_QUEUE_URL        | URL for the ingestion queue.                                                                                                                           |
| INGEST_DLQ_URL          | URL for the ingestion DLQ.                                                                                                                             |
| READWISE_WEBHOOK_SECRET | Verifies incoming Readwise webhook signatures. Prefix with `ssm:` to fetch from AWS SSM Parameter Store (e.g. `ssm:/ipp/dev/readwise/webhook_secret`). |
| ANTHROPIC_API_KEY       | Optional. If unset, LLM enrichment is disabled. Prefix with `ssm:` to fetch from AWS SSM Parameter Store (e.g. `ssm:/ipp/dev/anthropic/api_key`).      |

## Local ingest simulation

```bash
go run ./cmd/ingest-local/main.go
```

Prepared test payloads live in `./dev/http`
