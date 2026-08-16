---
id: ADR-013
title: LLM as Optional Enrichment
status: Accepted
date: 2026-01-04
related: [ADR-002, ADR-012, ADR-014]
---

# ADR-013: LLM as Optional Enrichment

## Decision

Treat the LLM as an optional, failure-tolerant enrichment step. An insight is persisted first and enriched second; enrichment failure degrades the result, never the write.

## Context

LLMs are powerful but:

- slow
- probabilistic
- cost-sensitive
- externally operated

Enrichment produces the tags that [ADR-012](012-single-table-design.md)'s index is built from, so it is valuable — but it is not what makes an insight worth keeping.

## Rationale

The system must remain correct and operational even if the LLM fails or is unavailable. Two mechanisms enforce that:

**Ordering.** `insight.Service.Process` writes the insight and publishes `InsightCreated` *before* calling the LLM. By the time enrichment can fail, the durable record already exists.

**Bounded calls.** The Anthropic client caps a request at 512 output tokens, 30 seconds, and 3 SDK retries. Those bounds are what let the worker's own 30s Lambda timeout ([ADR-002](002-serverless-first-compute.md)) hold.

## Consequences

- AI enrichment is best-effort. On failure the worker logs a warning and returns success with the insight stored unenriched — it does **not** retry the message, because the write already succeeded and a redelivery would short-circuit on the idempotency check anyway.
- Enrichment can be switched off entirely: when no API key resolves, the worker runs with a nil LLM service and skips the step. The pipeline is fully functional without an LLM configured.
- An unenriched insight carries no tags, so it is invisible to tag-scoped queries until something re-enriches it. There is no automatic re-enrichment pass — a gap worth closing if failure rates ever become non-trivial.
- Cost is bounded per call by construction, and bounded in aggregate only by how many insights are ingested.
- `InsightEnriched` is published only when enrichment succeeds ([ADR-014](014-domain-events-on-eventbridge.md)), so subscribers can treat it as a real signal rather than an attempt.
