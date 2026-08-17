# Setup (minimal)

## Environment variables

See [`.env.example`](../.env.example) for all variables, descriptions, and example values.

## OpenAI API key

One key covers every model capability in the system — LLM enrichment in the Go worker and
embeddings in the Python AI service (see [ADR-018](adr/018-one-provider-for-model-capabilities.md)).
Create one at **platform.openai.com → API keys**, then set `OPENAI_API_KEY` (prefix with `ssm:`
to fetch the value from AWS SSM Parameter Store instead — deployed Lambdas use
`ssm:/ipp/dev/openai/api_key`, a SecureString created out of band so the key never lands in
Terraform state).

Optional: leave it unset and enrichment is skipped rather than failing — the ingest pipeline
runs without it ([ADR-013](adr/013-llm-as-optional-enrichment.md)). Embeddings do require it.

## Raindrop.io token

Raindrop has no OAuth app registration step for local/demo use — get a non-expiring test token from **app.raindrop.io → Settings → Integrations**, then set `RAINDROP_API_TOKEN` (see `.env.example`; prefix with `ssm:` to fetch from AWS SSM Parameter Store instead of an env var).

## Local runners

**Readwise webhook server** (listens on `:8080`, accepts POST `/readwise/webhook`):

```bash
go run ./cmd/readwise-local
```

**REST API server** (listens on `:8081`, serves `POST /v1/readwise/import` and `POST /v1/raindrop/import`):

```bash
go run ./cmd/rest-local
```

**Raindrop poll simulator** (single pass against real Raindrop and SQS, then exits — requires `RAINDROP_API_TOKEN`):

```bash
go run ./cmd/raindrop-poll-local
```

**SQS worker simulator** (reads fixture from `cmd/worker-local/event.body.json`, runs once and exits):

```bash
go run ./cmd/worker-local
```

Prepared HTTP test payloads live in `./dev/http`.
