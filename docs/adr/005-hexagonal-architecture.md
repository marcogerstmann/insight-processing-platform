---
id: ADR-005
title: Hexagonal Architecture (Ports and Adapters)
status: Accepted
date: 2026-01-16
related: [ADR-001, ADR-006, ADR-010]
---

# ADR-005: Hexagonal Architecture (Ports and Adapters)

## Decision

Structure the Go code as ports and adapters:

```
internal/domain/       pure types and rules; no I/O, no AWS, no framework imports
internal/application/  use-case services (ingest, insight, llm, tenant)
internal/ports/        interfaces the application depends on
internal/adapters/
  inbound/             apigw, http/rest, schedule, sqs — things that call in
  outbound/            dynamodb, sqs, eventbridge, anthropic, readwise,
                       raindrop, ssm, memory — things the app calls out to
```

Ports live in one shared `internal/ports` package rather than being declared next to each consumer.

## Context

The system talks to at least eight external systems and is expected to grow more. Almost every one of them is a vendor-specific protocol (DynamoDB item shapes, SQS envelopes, Readwise webhook JSON, Raindrop pagination, Cognito JWKS). Without a boundary, those shapes leak into business logic and the code becomes untestable without AWS.

## Rationale

The direction of dependency is the whole point: `application` and `domain` never import an adapter, and adapters never import each other. Every crossing is an interface in `internal/ports`, small enough to state on one screen — `InsightRepository`, `EventPublisher`, `DomainEventPublisher`, `EnrichmentClient`, `HighlightSource`, `SecretProvider`, `DLQPublisher`.

This is what makes the rest of the system's claims testable rather than aspirational. `HighlightSource` having two production implementations ([ADR-010](010-multi-source-ingestion.md)) is the proof that the boundary is real, and the in-memory outbound adapters ([ADR-006](006-manual-di-and-paired-entrypoints.md)) are what let the whole pipeline run on a laptop.

Ports sit in one package because there are few of them and several have multiple consumers; the per-consumer-package convention would spread seven interfaces across five packages and add import cycles to work around. Revisit if the count grows enough that "who actually needs this port" stops being obvious.

## Consequences

- Every external shape needs an explicit mapping layer (`dto.go` / `mapper.go` in each inbound adapter). More files, more translation code, no ambiguity about where vendor JSON stops.
- Business logic is unit-testable with fakes and no AWS mocking library.
- `internal/ports` is a package every layer imports, so it must stay dependency-free apart from `internal/domain` — with one deliberate exception: `DLQPublisher` takes an `events.SQSMessage`, importing an AWS type into the port layer because forwarding a raw failed message is inherently transport-shaped.
- Swapping a vendor means writing one adapter, not editing the pipeline.
