---
id: ADR-007
title: Asynchronous Ingest via SQS
status: Accepted
date: 2026-01-04
related: [ADR-002, ADR-008, ADR-009, ADR-010]
---

# ADR-007: Asynchronous Ingest via SQS

## Decision

Put an SQS queue between the ingest edge and the processing worker. Edge functions validate, map, and enqueue; the worker consumes and does the slow work.

## Context

Upstream events (Readwise webhooks, Raindrop polls) are external, bursty, and outside system control. Downstream processing includes potentially slow or failing dependencies — chiefly the LLM enrichment call.

## Rationale

SQS provides:

- buffering during traffic spikes
- controlled retries
- backpressure when downstream systems are slow
- clear failure semantics via DLQ ([ADR-009](009-error-taxonomy-and-dlq-routing.md))

This prevents slow LLM calls from blocking ingestion and keeps the webhook endpoint fast enough that Readwise never sees a timeout.

## Consequences

- Increased latency compared to synchronous processing
- Clear separation between I/O concerns and domain logic
- Failure handling becomes explicit instead of implicit
- **`POST /v1/insights` is the exception.** The manual-create endpoint calls `insight.Service.Process` directly and does not enqueue, so it runs persistence *and* LLM enrichment inside the request. The caller is a human waiting on a form, one item at a time, and gets a synchronous answer about whether the write happened. Bulk paths (webhook, poll, `/readwise/import`, `/raindrop/import`) all go through the queue. If the manual path ever grows batch semantics, it should move behind the queue too.
- The event source mapping uses `batch_size = 1`, so an invocation handles exactly one message. This is what keeps the worker's batch-level retry behavior ([ADR-009](009-error-taxonomy-and-dlq-routing.md)) from re-processing healthy neighbors; raising the batch size without changing that code would reintroduce the problem.
