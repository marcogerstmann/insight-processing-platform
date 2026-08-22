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
                     a DynamoDB embedding reader/writer, OpenAI embeddings + relation labeling,
                     Cognito machine-to-machine auth, a relationship writer calling the Go API, DLQ)
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
of the highlight) plus its raw text, and stores the vector. Straight after that, in the same record, the
handler runs `application/relationship.py`'s `discover_relationships` — REL 2's candidate selection, REL 3's
LLM labeling, and REL 4's persistence through the Go API, one call each, see below. A malformed envelope, or
an event naming an insight that doesn't exist, raises `PermanentError`, caught here and routed to the
subscription's own DLQ; anything else — including an OpenAI API failure or a failed POST to the Go API —
propagates so the runtime redelivers, the same taxonomy as
`internal/adapters/inbound/sqs/worker.Handler` ([ADR-009](../../docs/adr/009-error-taxonomy-and-dlq-routing.md)).
Unlike Go's LLM enrichment ([ADR-013](../../docs/adr/013-llm-as-optional-enrichment.md)), neither embedding
nor relationship persistence is swallowed on failure — there's no already-durable write either is protecting,
so both are left to redeliver and, after this subscription's own retry budget, land on its DLQ. Redelivery is
safe: the embedding upsert, the Go endpoint's write, and (per-pair) the labeling call are all idempotent by
key, so a retry re-does work rather than duplicating it. Either way it can never block an insight write: this
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

## Relation labeling (IPP-99)

`application/relationship.py`'s `label_relationships` is REL 3: given a query `Insight` and the shortlist REL
2's `select_candidates` produced, ask an LLM what each candidate's relationship to the query actually is —
`adapters/outbound/openai.py`'s `OpenAiRelationLabeler`, one `/v1/chat/completions` call per pair, `relation_type`
constrained to a fixed enum (`supports`, `contradicts`, `extends`, `example_of`, `same_topic`) by a `strict`
JSON schema — the same substitution `internal/adapters/outbound/openai.Client` made for the Anthropic adapter's
forced tool choice (IPP-135), now applied to a second capability. A `relation_type` outside the enum is
rejected via `RelationType(...)` raising, never coerced.

Unlike embedding (IPP-97), a labeling failure is soft: there is no already-durable write it protects, so a
failing or below-`RELATIONSHIP_CONFIDENCE_THRESHOLD` pair is logged and skipped, and "no relationships found"
is a normal outcome rather than a DLQ-worthy one. `MAX_PAIRS_PER_RUN` bounds LLM calls per new insight,
separate from REL 2's `CANDIDATE_TOP_K`: candidate selection is free arithmetic and can afford to shortlist
generously, LLM calls can't. Model, token usage and call duration are logged with the same field names
(`model`, `input_tokens`, `output_tokens`, `duration_ms`) as the Go adapter's, so both services show up in one
Logs Insights query (IPP-113).

## Relationship persistence and the discovery pipeline (IPP-100)

`application/relationship.py`'s `discover_relationships` is what assembles REL 2 + REL 3 + persistence into
the one triggered flow the epic describes, and is what the Lambda handler calls right after `embed_insight`.
It loads the query insight, lists the tenant's stored embeddings (`ports.EmbeddingReader`,
`DynamoDbEmbeddingWriter.list_by_tenant` — the same adapter instance `embed_insight`'s writer uses, since it's
one table), runs `select_candidates`, labels the shortlist via `label_relationships`, and `put`s each accepted
`Relationship` through `ports.RelationshipWriter`. The concrete writer,
`adapters/outbound/relationship_api.py`'s `GoApiRelationshipWriter`, POSTs to
`/v1/tenants/:tenantID/insights/:insightID/relationships` using a `CognitoServiceTokenClient` bearer token —
idempotent server-side (re-posting the same edge updates it), so no client-side dedup is needed. `already_linked`
bookkeeping (skipping pairs that already have an edge) is intentionally not done here: `MAX_PAIRS_PER_RUN`
already bounds cost per run, and a redelivery just re-labels and idempotently overwrites the same edges rather
than duplicating them. Revisit only if a real corpus shows repeated LLM spend on already-linked pairs.

## Bounded context gathering for the Action Agent (PLAN 2, IPP-104)

A second domain event, `WeeklyPlanRequested` (emitted by the Go core when a user submits a weekly
focus — PLAN 1/IPP-103), lands on its **own** subscription queue —
`terraform/envs/dev/ai.tf`'s `action_agent_subscription`, isolated from `ai_subscription` above so a
failure gathering one tenant's weekly context never touches `InsightEnriched` processing's retries or
DLQ. Both queues trigger the same Lambda function via two separate event source mappings — one Python
package, one deployed image, `adapters/inbound/event_subscription.py`'s `Handler.handle` branches on
`event_type` rather than a second container.

The obvious implementation — "send every insight tagged with the focus tag to the LLM" — works for a
handful of highlights and breaks (unpredictable cost, a context window that doesn't fit) once a tag
holds hundreds. `domain/plan_context.py`'s `select_context` is the deterministic, pure answer to
_what not to send_: it loads a tag's insights (TAG 4) and every one of their relationship edges
(REL 6), ranks them by a weighted recency + relationship-centrality score — the same
exponential-half-life-decay, saturating-count shape as `internal/domain.TagRelevanceScore`, translated
from tag-level ranking to insight-level selection — then admits ranked insights one at a time under two
independent caps: `MAX_SELECTED_INSIGHTS` and a hard `CONTEXT_TOKEN_BUDGET`. The budget truncates by
dropping whole insights, never mid-text: an insight that doesn't fit is skipped, not clipped, so a
smaller one further down the ranking may still make the cut. Relationships between two insights that
both survive selection are carried along as `ContextEdge`s — the whole reason the graph exists is
that a contradiction between two selected insights is exactly the tension worth an action, and the LLM
(PLAN 3) can't see it without the edge. A tag with fewer than `MIN_INSIGHTS_FOR_PLAN` insights — before
or after budget truncation — returns `InsufficientMaterial` instead: a plan needs a handful of distinct
threads to draw 3-5 actions from, and PLAN 3 must never be handed a corpus thin enough to invite a
hallucinated one.

`application/context_gathering.py`'s `gather_context` is the thin orchestration `Handler.handle` calls:
one `list_by_tag` (`ports.InsightReader`) plus one `list_by_insight` per insight in the tag
(`ports.RelationshipReader`, satisfied by the same `DynamoDbInsightReader` class that satisfies
`InsightReader` — one class, one table, same read-only boundary), then `select_context`. Every outcome
is logged — selected insight ids and the token budget used on success, tag and insight count on
`InsufficientMaterial` — so the input to any plan is reconstructable after the fact even though nothing
downstream persists this context yet. PLAN 3 is the next story: it calls `select_context`'s result to
make the actual LLM call, and PLAN 4 persists the result the `202` from PLAN 1 is waiting on.

## Machine-to-machine auth (IPP-94)

`adapters/outbound/cognito.py`'s `CognitoServiceTokenClient` is AI 3, the Python half of
[ADR-019](../../docs/adr/019-machine-to-machine-auth-for-agent-writes.md): this service authenticates against
the Go REST API as itself, via a Cognito app client scoped to Cognito's OAuth2 `client_credentials` grant
(`terraform/envs/dev/rest-api.tf`'s `aws_cognito_user_pool_client.agent`), not as any user. `token()` fetches
and caches the access token until shortly before it expires, refreshing on demand rather than once per
request. The client secret follows the same `ssm:`-prefixed convention as every other secret here
(`application/secrets.py`), populated by Terraform because — unlike the OpenAI key — Cognito generates it as
part of a Terraform-managed resource, so it's already in state regardless.

`GoApiRelationshipWriter` (above) is the caller: it takes a `token()`-shaped object as a structural type
rather than importing this module directly — an adapter importing another adapter is banned by the same
ruff rule that keeps domain/ports/application from importing adapters (ADR-017) — so only the composition
root (`lambda_handler`) imports and wires the two together.

## Dev

```bash
cd services/ai
uv sync            # install deps + create .venv
uv run pytest      # or: make ai-test    (from repo root)
uv run ruff check . && uv run ruff format --check .   # or: make ai-lint
uv run python -m ipp_ai                                # or: make ai-run-local
```
