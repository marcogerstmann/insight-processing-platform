# Setup (minimal)

## Environment variables

See [`.env.example`](../.env.example) for all variables, descriptions, and example values.

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
