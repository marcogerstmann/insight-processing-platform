---
id: ADR-001
title: AWS as Cloud Platform
status: Accepted
date: 2026-01-04
related: [ADR-002, ADR-003, ADR-004]
---

# ADR-001: AWS as Cloud Platform

## Decision

Use AWS as the cloud provider.

## Context

The system requires managed primitives for:

- event ingestion
- asynchronous processing
- retry handling
- cost visibility
- minimal operational overhead

## Rationale

AWS provides mature, well-understood building blocks (Lambda, SQS, DynamoDB, EventBridge) that map directly to the system's needs. The platform enables fine-grained cost control and avoids maintaining undifferentiated infrastructure.

## Consequences

- Strong alignment with serverless and event-driven patterns
- Vendor lock-in is accepted; migration would require rethinking ingestion, queuing, and persistence layers
- Architecture favors managed services over custom runtime control
- The lock-in is bounded by [ADR-005](005-hexagonal-architecture.md): AWS SDK types stay in `internal/adapters/`, so the domain and application layers are portable even though the infrastructure is not
