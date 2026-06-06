# Setup (minimal)

## Environment variables

See [`.env.example`](../.env.example) for all variables, descriptions, and example values.

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
