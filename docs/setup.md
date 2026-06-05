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

## Local runners

**Readwise webhook server** (listens on `:8080`, accepts POST `/readwise/webhook`):

```bash
go run ./cmd/readwise-local
```

**REST API server** (listens on `:8081`):

```bash
go run ./cmd/rest-local
```

**SQS worker simulator** (reads fixture from `cmd/worker-local/event.body.json`, runs once and exits):

```bash
go run ./cmd/worker-local
```

Prepared HTTP test payloads live in `./dev/http`.
