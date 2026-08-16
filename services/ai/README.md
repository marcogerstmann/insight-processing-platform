# ipp-ai

The Python half of the Insight Processing Platform: an event-driven service that reacts to domain events
published by the Go core (`InsightEnriched`, ...) to build the knowledge graph and weekly Action Agent
described in the `vision-backlog` epics. It is a **subscriber and reader**, not a second front door — writes
back into the domain go through the Go REST API (agent-scoped, machine-to-machine auth), never directly to
DynamoDB (why two languages at all is a separate ADR, still to come). See
[ADR-017](../../docs/adr/017-idiomatic-python-in-the-ai-service.md) for why this service's code doesn't look
like Go transliterated into Python even though it keeps the same hexagonal boundaries.

Deliberately no web framework — there is no inbound HTTP surface here, only an event handler (arriving in
[IPP-95](https://marcogerstmann.atlassian.net/browse/IPP-95)).

```
src/ipp_ai/
  domain/            pure types and rules; no I/O, no AWS, no framework imports
  ports.py           typing.Protocol definitions the application depends on
  application/       use-case orchestration; reads env vars directly (see below)
  adapters/outbound/ things the service calls out to (SSM, and a read-only DynamoDB insight reader)
  errors.py          PermanentError: permanent failure -> DLQ, everything else -> retry
```

`adapters/inbound/` isn't created yet — there's nothing inbound until the event handler in IPP-95 exists to
put there. An empty package for shape alone is exactly what [ADR-017](../../docs/adr/017-idiomatic-python-in-the-ai-service.md)
argues against.

## Config and secrets

Same convention as the Go services (`internal/envutil.ResolveSecret`): one env var per secret, holding either
the value directly (local dev) or an `ssm:`-prefixed SSM parameter path (AWS). See `application/secrets.py`.

## Read-only access to the insights table

`adapters/outbound/dynamodb.py` (`DynamoDbInsightReader`, satisfying `ports.InsightReader`) is this service's
only path to the insights table, and it only ever calls `GetItem` / `Query`. There is no write path here on
purpose: **the Lambda execution role this adapter runs under (wired up in IPP-95) must grant only `GetItem`
and `Query` on the table and its `gsi1` index** — no `PutItem`, `UpdateItem`, or `DeleteItem`. Enforcing that
at IAM makes the read-only boundary a property of the infrastructure, not a code-review convention. Writes
back into the domain go through the Go REST API instead (IPP-94).

Key schema (`pk = TENANT#<tenant_id>`, `sk = INSIGHT#<id>`, tag membership queried via the sparse `gsi1`
index) is copied from `internal/adapters/outbound/dynamodb/insight_adapter.go` — two languages now unmarshal
the same items, so a schema change there is a two-repo-location change, not a one-line diff.

## Dev

```bash
cd services/ai
uv sync            # install deps + create .venv
uv run pytest      # or: make ai-test    (from repo root)
uv run ruff check . && uv run ruff format --check .   # or: make ai-lint
uv run python -m ipp_ai                                # or: make ai-run-local
```
