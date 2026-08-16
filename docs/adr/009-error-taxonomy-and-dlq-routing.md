---
id: ADR-009
title: Error Taxonomy and Explicit DLQ Routing
status: Accepted
date: 2026-06-04
related: [ADR-007, ADR-008]
---

# ADR-009: Error Taxonomy and Explicit DLQ Routing

## Decision

Classify every worker failure as either permanent or transient, and act on the classification:

- **Permanent** (`apperr.PermanentError`) — malformed message, missing source, missing highlight ID. Send the record to the DLQ immediately via `ports.DLQPublisher` and continue to the next record.
- **Transient** — everything else (DynamoDB throttling, LLM outage, EventBridge failure). Return the error so SQS redelivers.

Poison messages are routed by application code, not left to the queue's `maxReceiveCount` redrive policy alone.

## Context

SQS's built-in redrive treats every failure the same way: retry N times, then move to the DLQ. A message missing a required field will never succeed, but redrive still burns the full retry budget on it — consuming invocations, delaying the queue, and turning a deterministic bug into a several-minute wait before anyone can see it.

## Rationale

The information about *why* a message failed only exists inside the handler. `mapMessageDTOToDomain` knows that a missing `highlight.ID` is unfixable; SQS cannot know that. Routing on that knowledge means a bad message lands in the DLQ on its first sight, with the failure reason attached, while a genuine outage still gets the full retry treatment.

Returning the error for transient failures rather than swallowing it is what preserves at-least-once delivery, and [ADR-008](008-idempotency-via-deterministic-key.md) is what makes those retries safe.

## Consequences

- Poison messages surface in the DLQ in seconds with a reason, instead of after the redrive budget expires with none.
- Every new failure path forces an explicit call: is this worth retrying? Forgetting to wrap a permanent failure in `apperr.PermanentError` degrades it to a retry loop — noisy, but not lossy.
- Returning an error fails the **whole batch**, not the single record. Safe today only because `batch_size = 1` ([ADR-007](007-asynchronous-ingest-via-sqs.md)); raising it requires switching to partial batch responses (`ReportBatchItemFailures`) first.
- A DLQ send that itself fails is logged and dropped — the record then falls back to ordinary redrive, which is the acceptable degraded path.
