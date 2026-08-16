# Architecture Decision Records

This document captures the **core architectural decisions** of the Insight Processing Platform.

The goal is not exhaustiveness, but **defensibility**: each decision is intentional, constrained, and reversible only with a clear trade-off.

---

## ADR-001: AWS as Cloud Platform

### Decision

Use AWS as the cloud provider.

## Context

The system requires managed primitives for:

- event ingestion
- asynchronous processing
- retry handling
- cost visibility
- minimal operational overhead

### Rationale

AWS provides mature, well-understood building blocks (Lambda, SQS, DynamoDB) that map directly to the system's needs. The platform enables fine-grained cost control and avoids maintaining undifferentiated infrastructure.

### Consequences

- Strong alignment with serverless and event-driven patterns
- Vendor lock-in is accepted; migration would require rethinking ingestion, queuing, and persistence layers
- Architecture favors managed services over custom runtime control

---

## ADR-002: Ingest & Decoupling with API Gateway + Lambda + SQS

### Decision

Use API Gateway and a lightweight ingest Lambda to receive events, and SQS to decouple ingestion phase from processing.

### Context

Upstream events (e.g. Readwise webhooks) are external, bursty, and outside system control. Downstream processing includes potentially slow or failing dependencies (LLMs).

### Rationale

SQS provides:

- buffering during traffic spikes
- controlled retries
- backpressure when downstream systems are slow
- clear failure semantics via DLQ

This prevents slow LLM calls from blocking ingestion and keeps the system responsive under load.

### Consequences

- Increased latency compared to synchronous processing
- Clear separation between I/O concerns and domain logic
- Failure handling becomes explicit instead of implicit

---

## ADR-003: Serverless First Compute Model

### Decision

Use AWS Lambda for ingestion and for the core processing worker (via container image). The ingest Lambda enqueues events to SQS, the processing worker consumes from SQS.

### Context

The system has low to moderate traffic, unpredictable load, and no strict latency requirements for background processing.

### Rationale

Lambda minimizes idle cost, removes server management, and fits naturally with event-driven workflows. Container images allow use of Go with full dependency control while retaining serverless benefits.

### Consequences

- Cold starts are accepted
- Execution time limits shape processing logic
- Horizontal scaling is automatic and managed

---

## ADR-004: No Kubernetes

### Decision

Do not use Kubernetes.

### Context

Traffic volume is low, system size is small, and operational simplicity is a priority.

### Rationale

Kubernetes would introduce:

- control-plane cost
- operational complexity
- additional failure modes

These costs are unjustified for the problem domain. Lambda and managed services provide sufficient scalability and reliability.

### Consequences

- Less flexibility in runtime customization
- Stronger coupling to AWS primitives
- Lower operational burden

---

## ADR-005: Data Store

### Decision

Use DynamoDB with on-demand capacity for persistence.

### Context

The data model is event-oriented, access patterns are known upfront, and traffic volume is low.

### Rationale

DynamoDB provides:

- zero idle cost
- predictable scaling
- strong integration with Lambda
- schema flexibility suitable for evolving insight structures

A relational database would introduce unnecessary operational and cost overhead.

### Consequences

- Access patterns must be designed explicitly
- Limited ad-hoc querying
- Data modeling effort shifts to design time

---

## ADR-006: LLM Usage

### Decision

Treat LLMs as an optional, failure-tolerant enrichment step.

### Context

LLMs are powerful but:

- slow
- probabilistic
- cost-sensitive
- externally operated

### Rationale

The system must remain correct and operational even if LLMs fail or are unavailable. Hard limits on tokens, timeouts, and retries ensure predictable cost and behavior.

### Consequences

- AI enrichment is best-effort
- Fallback processing is mandatory
- Cost overruns are prevented by design

---

## ADR-007: Idempotency Strategy

### Decision

Ensure idempotent ingestion and processing using a deterministic event key.

### Context

Webhook deliveries may be retried or duplicated by upstream systems. SQS guarantees at-least-once delivery.

### Rationale

Idempotency prevents duplicate processing and inconsistent state without relying on exactly-once semantics.

### Consequences

- Duplicate events are safely ignored
- State checks are required before persistence
- Storage schema must support idempotency keys

---

## ADR-008: Multi-Source Ingestion

### Decision

Add Raindrop.io as a second highlight source alongside Readwise, behind the same `ports.HighlightSource` port. Raindrop is polled on a schedule rather than pushed via webhook. Polling uses no cursor or watermark. Readwise stays wired up rather than being replaced.

### Context

The Readwise paid subscription was cancelled; Raindrop.io was chosen as its replacement. Rather than a straight swap, Raindrop was added alongside Readwise so the system's source-agnosticism claim (see the README) is demonstrated by two live adapters instead of asserted with one.

### Rationale

**Poll, not webhook.** Raindrop's API has no push webhook. The ingest edge stays event-driven either way — SQS, the worker, and everything downstream is identical — only the trigger transport differs: Readwise's Lambda is invoked by API Gateway, Raindrop's by EventBridge Scheduler on a recurring cadence.

**No cursor or watermark.** The poll re-fetches recent highlights every run and relies on the existing idempotency key (ADR-007) to dedupe at the DynamoDB layer, instead of persisting a `last_polled_at` cursor. This is deliberate, not a shortcut dressed up as one: Raindrop's `page`/`perpage` pagination is offset-based, so a concurrent write during a poll can duplicate or skip an item at the page boundary. Duplicates are already free to absorb via the idempotency key; anything skipped is picked up by the next run. A cursor would add state to corrupt or migrate and would still not fix the skip case, so it buys nothing.

**Readwise stays.** Keeping both sources live, rather than migrating and deleting the Readwise adapter, is what makes the port abstraction provable rather than aspirational — a second adapter that never shipped would leave "source-agnostic" as an unverified claim.

### Consequences

- A highlight imported via the REST API, Readwise's webhook, or a Raindrop poll all hash to the same idempotency key and dedupe against each other, regardless of which path delivered it first.
- Raindrop's free tier caps highlights at **3 per bookmark** (bookmarks and total highlights are otherwise unlimited). Accepted as a known limitation of the demo token; Raindrop Pro ($3/mo) removes it if it ever binds.
- `SourceTitle` on `domain.Highlight` is deliberately deferred: Raindrop's API returns a `title` field that Readwise's does not, and it is currently dropped rather than partially modeled. Add it if a source's title becomes load-bearing for a downstream feature.
- Kindle's `My Clippings.txt` was considered and deferred as a third source. Unlike Readwise and Raindrop it has no stable per-highlight ID and uses locale-dependent date formats — real enough gotchas to warrant its own story rather than folding into this port's assumptions.
