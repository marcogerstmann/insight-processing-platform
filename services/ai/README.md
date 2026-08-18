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
  adapters/outbound/ things the service calls out to (SSM, a read-only DynamoDB insight reader,
                     a DynamoDB embedding writer, OpenAI embeddings, DLQ)
  errors.py          PermanentError: permanent failure -> DLQ, everything else -> retry
  logging_config.py  structured JSON logging, field names matching the Go service's
```

## Deployment and the domain-event subscription

`services/ai/Dockerfile` builds a Lambda container image (`public.ecr.aws/lambda/python`, via `uv`). Terraform
(`terraform/envs/dev/ai.tf`) provisions the ECR repo, subscribes to `InsightEnriched` on the domain events bus
via `terraform/modules/event-subscription` (its own SQS queue + DLQ, isolated from every other subscriber —
see [ADR-014](../../docs/adr/014-domain-events-on-eventbridge.md)), and wires that queue to the Lambda with an
event source mapping.

`adapters/inbound/event_subscription.py`'s `Handler` did the minimum IPP-95 asked for: unmarshal the
EventBridge envelope into a `domain.event.DomainEvent`, log it structurally (`tenant_id`, `insight_id`,
`event_type` — matching the Go service's field names). IPP-97 adds the first real step after that:
`application/embedding.py`'s `embed_insight` loads the insight the event names, embeds its tags (a summary
of the highlight) plus its raw text, and stores the vector. A malformed envelope, or an event naming an
insight that doesn't exist, raises `PermanentError`, caught here and routed to the subscription's own DLQ;
anything else — including an OpenAI API failure — propagates so the runtime redelivers, the same taxonomy as
`internal/adapters/inbound/sqs/worker.Handler` ([ADR-009](../../docs/adr/009-error-taxonomy-and-dlq-routing.md)).
Unlike Go's LLM enrichment ([ADR-013](../../docs/adr/013-llm-as-optional-enrichment.md)), embedding failure
isn't swallowed — there's no already-durable write it's protecting, so it's left to redeliver and, after this
subscription's own retry budget, land on its DLQ. Either way it can never block an insight write: this
service is a subscriber off to the side of the Go core.
The event source mapping's `batch_size` must stay `1` for the same reason the Go worker's does: this
per-record loop only fails-safe with one record per invocation.

`make ai-build` / `make ai-push` mirror `worker-build` / `worker-push`; `make deploy` runs `ai-deploy`, which
skips both when `AI_TAG` (the last commit that touched `services/ai`) is already in the immutable ECR repo —
a push that doesn't touch this service reuses its existing image instead of pushing a new one. `worker-deploy`
guards the worker's build+push the same way, for a different reason: both repos are IMMUTABLE, so re-running
a deploy that already pushed an image (e.g. after a later step like `tf-apply` failed) would otherwise try to
overwrite that tag and get rejected. Both build+push steps run only in CI
([`deploy.yml`](../../.github/workflows/deploy.yml)) — nobody's laptop uploads a production image. `ruff
check`/`ruff format --check`/`pytest` (`make ai-lint` / `make ai-test`) run on every push and gate the deploy
job, same as the Go service.

## Config and secrets

Same convention as the Go services (`internal/envutil.ResolveSecret`): one env var per secret, holding either
the value directly (local dev) or an `ssm:`-prefixed SSM parameter path (AWS). See `application/secrets.py`.

## Read-only access to the insights table

`adapters/outbound/dynamodb.py` (`DynamoDbInsightReader`, satisfying `ports.InsightReader`) is this service's
only path to the insights table, and it only ever calls `GetItem` / `Query`. There is no write path here on
purpose: **the Lambda execution role (`terraform/envs/dev/ai.tf`) grants only `GetItem` and `Query`** on the
table and its `gsi1` index — no `PutItem`, `UpdateItem`, or `DeleteItem`. Enforcing that at IAM makes the
read-only boundary a property of the infrastructure, not a code-review convention. Writes back into the
domain go through the Go REST API instead (IPP-94).

Key schema (`pk = TENANT#<tenant_id>`, `sk = INSIGHT#<id>`, tag membership queried via the sparse `gsi1`
index) is copied from `internal/adapters/outbound/dynamodb/insight_adapter.go` — two languages now unmarshal
the same items, so a schema change there is a two-repo-location change, not a one-line diff.

## Embeddings (IPP-97)

`InsightEnriched` triggers `embed_insight`: load the insight, embed its tags plus its text via
`ports.EmbeddingClient`, store the result via `ports.EmbeddingWriter`. The provider is
`adapters/outbound/openai.py`'s `OpenAiEmbeddingClient` — `text-embedding-3-small`, requested at 512
dimensions rather than its 1536 default, because vectors are compared by brute force and stored as DynamoDB
`Decimal` lists, so width sets item size, read cost and comparison cost at once
([ADR-018](../../docs/adr/018-one-provider-for-model-capabilities.md)). Nothing outside that one file knows
the provider: the port is what made replacing Voyage with OpenAI a single-file change. It's a plain
`urllib.request` call, bounded the same way the Go worker's OpenAI client
is ([ADR-013](../../docs/adr/013-llm-as-optional-enrichment.md)'s discipline, applied to a second capability):
a per-attempt timeout, capped retries (skipped on a 4xx — retrying a bad key or a bad request changes
nothing), and the input truncated before it's sent, all bounded in total to fit inside the Lambda's own 30s
timeout alongside the DynamoDB calls either side of it.

Vectors land in **this service's own table** (`dynamodb_ai_embeddings` in `terraform/envs/dev/ai.tf`), not
the shared insights table — `pk = TENANT#<tenant_id>`, `sk = EMBEDDING#<insight_id>`, alongside the model
name and dimension so a future model change is detectable instead of silently mixing incompatible vector
spaces. `PutItem` with no condition, so re-processing the same event overwrites in place — the same
deterministic-key idempotency as everywhere else in this codebase
([ADR-008](../../docs/adr/008-idempotency-via-deterministic-key.md)).

## Candidate selection (IPP-98)

`domain/candidate.py`'s `select_candidates` is REL 2: given a query embedding and a tenant's stored
embeddings, return the top-`CANDIDATE_TOP_K` by cosine similarity above `CANDIDATE_SIMILARITY_THRESHOLD`,
excluding the query insight itself and any `already_linked` ids the caller passes in. Pure and I/O-free by
design — loading the tenant's vectors is an adapter's job, not this function's — so REL 3's agent can call it
without a test needing DynamoDB. The brute-force scan is the same deliberate "no vector database" call as
`domain/embedding.py`'s `cosine_similarity`, just vectorized: `numpy` computes one batched dot product against
every candidate instead of a Python loop, which is the only reason this service takes a `numpy` dependency.

## Dev

```bash
cd services/ai
uv sync            # install deps + create .venv
uv run pytest      # or: make ai-test    (from repo root)
uv run ruff check . && uv run ruff format --check .   # or: make ai-lint
uv run python -m ipp_ai                                # or: make ai-run-local
```
