# ipp-ai

The Python half of the Insight Processing Platform: an event-driven service that reacts to domain events
published by the Go core (`InsightEnriched`, ...) to build the knowledge graph and weekly Action Agent
described in the `vision-backlog` epics. It is a **subscriber and reader**, not a second front door — writes
back into the domain go through the Go REST API (agent-scoped, machine-to-machine auth), never directly to
DynamoDB (why two languages at all is a separate ADR, still to come). See
[ADR-017](../../docs/adr/017-idiomatic-python-in-the-ai-service.md) for why this service's code doesn't look
like Go transliterated into Python even though it keeps the same hexagonal boundaries.

Deliberately no web framework — there is no inbound HTTP surface here, only an EventBridge subscription
delivered over SQS (`adapters/inbound/event_subscription.py`).

```
src/ipp_ai/
  domain/            pure types and rules; no I/O, no AWS, no framework imports
  ports.py           typing.Protocol definitions the application depends on
  application/       use-case orchestration; reads env vars directly (see below)
  adapters/inbound/  things that call into the service (the EventBridge subscription handler)
  adapters/outbound/ things the service calls out to (SSM, a read-only DynamoDB insight reader, DLQ)
  errors.py          PermanentError: permanent failure -> DLQ, everything else -> retry
  logging_config.py  structured JSON logging, field names matching the Go service's
```

## Deployment and the domain-event subscription

`services/ai/Dockerfile` builds a Lambda container image (`public.ecr.aws/lambda/python`, via `uv`). Terraform
(`terraform/envs/dev/ai.tf`) provisions the ECR repo, subscribes to `InsightEnriched` on the domain events bus
via `terraform/modules/event-subscription` (its own SQS queue + DLQ, isolated from every other subscriber —
see [ADR-014](../../docs/adr/014-domain-events-on-eventbridge.md)), and wires that queue to the Lambda with an
event source mapping.

`adapters/inbound/event_subscription.py`'s `Handler` does the minimum IPP-95 asks for: unmarshal the
EventBridge envelope into a `domain.event.DomainEvent`, log it structurally (`tenant_id`, `insight_id`,
`event_type` — matching the Go service's field names), done. A malformed envelope raises `PermanentError`,
caught here and routed to the subscription's own DLQ; anything else propagates so the runtime redelivers —
the same taxonomy as `internal/adapters/inbound/sqs/worker.Handler` ([ADR-009](../../docs/adr/009-error-taxonomy-and-dlq-routing.md)).
The event source mapping's `batch_size` must stay `1` for the same reason the Go worker's does: this
per-record loop only fails-safe with one record per invocation.

`make ai-build` / `make ai-push` mirror `worker-build` / `worker-push`; `make deploy` runs `ai-deploy`, which
skips both when `AI_TAG` (the last commit that touched `services/ai`) is already in the immutable ECR repo —
a push that doesn't touch this service reuses its existing image instead of pushing a new one. Both build+push
steps run only in CI ([`deploy.yml`](../../.github/workflows/deploy.yml)) — nobody's laptop uploads a
production image. `ruff check`/`ruff format --check`/`pytest` (`make ai-lint` / `make ai-test`) run on every
push and gate the deploy job, same as the Go service.

## Config and secrets

Same convention as the Go services (`internal/envutil.ResolveSecret`): one env var per secret, holding either
the value directly (local dev) or an `ssm:`-prefixed SSM parameter path (AWS). See `application/secrets.py`.

## Read-only access to the insights table

`adapters/outbound/dynamodb.py` (`DynamoDbInsightReader`, satisfying `ports.InsightReader`) is this service's
only path to the insights table, and it only ever calls `GetItem` / `Query`. There is no write path here on
purpose: **the Lambda execution role (`terraform/envs/dev/ai.tf`) grants only `GetItem` and `Query`** on the
table and its `gsi1` index — no `PutItem`, `UpdateItem`, or `DeleteItem`. Enforcing that at IAM makes the
read-only boundary a property of the infrastructure, not a code-review convention. Writes back into the
domain go through the Go REST API instead (IPP-94). Nothing calls this adapter yet — IPP-95's handler does no
more than log an event — the grant exists ahead of its first caller because there is exactly one execution
role for the whole service.

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
