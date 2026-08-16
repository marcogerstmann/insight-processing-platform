---
id: ADR-008
title: Idempotency via a Deterministic Event Key
status: Accepted
date: 2026-01-18
related: [ADR-007, ADR-010, ADR-012, ADR-014]
---

# ADR-008: Idempotency via a Deterministic Event Key

## Decision

Derive an ingest event's identity from its content, not from the delivery:

```
id = sha256(tenantID | source | eventType | highlight.ID)
```

That hash becomes the `IngestEvent.ID`, the `insight.ID`, and the DynamoDB sort key. Deduplication is enforced at write time by a conditional put, not by a prior read.

## Context

Webhook deliveries may be retried or duplicated by upstream systems. SQS guarantees at-least-once delivery. The Raindrop poll deliberately re-fetches overlapping windows on every run ([ADR-010](010-multi-source-ingestion.md)). All three produce duplicate work that must not produce duplicate records.

## Rationale

A content-derived key makes duplicate suppression a property of the data rather than of the pipeline. Any path that delivers the same highlight computes the same key, so the paths do not need to know about each other.

Enforcement is a single `CreateIfAbsent` with `ConditionExpression: attribute_not_exists(pk)`. A check-then-write would race under concurrent redelivery; the conditional write is atomic in DynamoDB and returns `inserted=false` instead of an error, which the worker treats as success and stops.

Domain events use the same trick with a different input — `sha256(eventType | subjectID)` — so a redelivered SQS message republishes an event carrying the identical `event_id` and subscribers can dedupe on it ([ADR-014](014-domain-events-on-eventbridge.md)).

## Consequences

- A highlight arriving via the Readwise webhook, `/readwise/import`, or a Raindrop poll dedupes against itself regardless of which path delivered it first — provided all of them stamp the same `source` and `eventType`, which `ingest.Importer` exists to guarantee.
- Editing a highlight's text upstream does not change its key, so **updates are not re-ingested** — the first version wins. Acceptable while highlights are treated as immutable captures; a real edit story would need a version or content hash in the key.
- **Manual insights are not idempotent.** `POST /v1/insights` assigns `uuid.New()` because free-form text has no natural key — posting the same body twice creates two records. This is the deliberate boundary of the guarantee: it covers source-derived highlights, not user-authored ones.
- The storage schema must carry the key as its sort key, which couples [ADR-012](012-single-table-design.md)'s key design to this decision.
