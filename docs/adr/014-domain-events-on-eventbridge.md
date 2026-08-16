---
id: ADR-014
title: Domain Events on EventBridge
status: Accepted
date: 2026-08-13
related: [ADR-007, ADR-008, ADR-013]
---

# ADR-014: Domain Events on EventBridge

## Decision

Publish domain events to a dedicated EventBridge bus, separate from the SQS work queue. The queue carries pipeline transport; the bus carries facts.

Events are versioned envelopes defined in `internal/domain`, free of any AWS concept:

```go
type DomainEvent struct {
    EventID    string    // sha256(eventType | subjectID)
    EventType  EventType // InsightCreated, InsightEnriched, ...
    Version    int       // envelope schema version, starts at 1
    TenantID   string
    OccurredAt time.Time
    Payload    any
}
```

The outbound adapter maps `EventType` to EventBridge's `DetailType` and stamps `Source: "ipp.core"`.

## Context

The ingest queue already moves messages between two components of one pipeline. Reusing it for "something happened in the domain" would mean subscribers filtering pipeline traffic to find facts, and any new subscriber changing the delivery semantics of ingestion.

## Rationale

**Two channels, two jobs.** SQS is point-to-point work distribution with retries and a DLQ; one consumer drains it. EventBridge is fan-out: N subscribers, content-based routing, no coupling between them. The two ports stay distinct in code for the same reason — `EventPublisher` (raw bytes, work queue) versus `DomainEventPublisher` (typed fact).

**A transport-free envelope.** Keeping `DomainEvent` in `internal/domain` means the event contract belongs to the domain, and switching bus technology touches one adapter ([ADR-005](005-hexagonal-architecture.md)).

**Deterministic event IDs.** `sha256(eventType | subjectID)` means a redelivered SQS message republishes the same `event_id`, so subscribers can dedupe on it — the same reasoning as [ADR-008](008-idempotency-via-deterministic-key.md), applied one layer out.

**Thin payloads.** `InsightCreatedPayload` carries an ID and a source, not the insight body. Subscribers read what they need; the event stays a notification rather than a replication channel.

## Consequences

- **Nothing subscribes to the bus yet.** No EventBridge rules are defined, and `KnowledgeUpdated` is declared but never published. This is infrastructure built ahead of its consumers — deliberate, since the knowledge-graph work is what it exists for, but it is unproven until the first subscriber lands.
- Publishing is a dual write with no transaction: the insight is stored, then the event is published. A publish failure returns a transient error so SQS redelivers, but on redelivery `CreateIfAbsent` short-circuits before the publish is reached — so a persistently failing bus drops the event while keeping the write. Closing that gap needs a published flag or an outbox table; the shortcut is marked in `insight/service.go` and is not worth paying for until it bites.
- Adding a subscriber is a Terraform rule, not a code change in the publisher.
- EventBridge appears twice in this system for unrelated reasons: this bus, and EventBridge Scheduler as the Raindrop poll trigger ([ADR-010](010-multi-source-ingestion.md)). They share a service name and nothing else.
