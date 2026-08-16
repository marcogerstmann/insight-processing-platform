---
id: ADR-010
title: Multi-Source Ingestion
status: Accepted
date: 2026-08-16
related: [ADR-005, ADR-007, ADR-008]
---

# ADR-010: Multi-Source Ingestion

## Decision

Add Raindrop.io as a second highlight source alongside Readwise, behind the same `ports.HighlightSource` port. Raindrop is polled on a schedule rather than pushed via webhook. Polling uses no cursor or watermark. Readwise stays wired up rather than being replaced.

## Context

The Readwise paid subscription was cancelled; Raindrop.io was chosen as its replacement. Rather than a straight swap, Raindrop was added alongside Readwise so the system's source-agnosticism claim (see the README) is demonstrated by two live adapters instead of asserted with one.

## Rationale

**Poll, not webhook.** Raindrop's API has no push webhook. The ingest edge stays event-driven either way — SQS, the worker, and everything downstream is identical — only the trigger transport differs: Readwise's Lambda is invoked by API Gateway, Raindrop's by EventBridge Scheduler on a recurring cadence.

**No cursor or watermark.** The poll re-fetches recent highlights every run and relies on the existing idempotency key ([ADR-008](008-idempotency-via-deterministic-key.md)) to dedupe at the DynamoDB layer, instead of persisting a `last_polled_at` cursor. This is deliberate, not a shortcut dressed up as one: Raindrop's `page`/`perpage` pagination is offset-based, so a concurrent write during a poll can duplicate or skip an item at the page boundary. Duplicates are already free to absorb via the idempotency key; anything skipped is picked up by the next run. A cursor would add state to corrupt or migrate and would still not fix the skip case, so it buys nothing.

**Readwise stays.** Keeping both sources live, rather than migrating and deleting the Readwise adapter, is what makes the port abstraction of [ADR-005](005-hexagonal-architecture.md) provable rather than aspirational — a second adapter that never shipped would leave "source-agnostic" as an unverified claim.

## Consequences

- A highlight imported via `/readwise/import`, `/raindrop/import`, Readwise's webhook, or a Raindrop poll all hash to the same idempotency key and dedupe against each other, regardless of which path delivered it first. `ingest.Importer` stamps `source` and `eventType` to keep that true.
- Raindrop's free tier caps highlights at **3 per bookmark** (bookmarks and total highlights are otherwise unlimited). Accepted as a known limitation of the demo token; Raindrop Pro ($3/mo) removes it if it ever binds.
- `SourceTitle` on `domain.Highlight` is deliberately deferred: Raindrop's API returns a `title` field that Readwise's does not, and it is currently dropped rather than partially modeled. Add it if a source's title becomes load-bearing for a downstream feature.
- Kindle's `My Clippings.txt` was considered and deferred as a third source. Unlike Readwise and Raindrop it has no stable per-highlight ID and uses locale-dependent date formats — real enough gotchas to warrant its own story rather than folding into this port's assumptions.
