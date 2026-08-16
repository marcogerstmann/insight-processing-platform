---
id: ADR-011
title: DynamoDB as Data Store
status: Accepted
date: 2026-01-04
related: [ADR-001, ADR-012]
---

# ADR-011: DynamoDB as Data Store

## Decision

Use DynamoDB with on-demand capacity (`PAY_PER_REQUEST`) for persistence, with point-in-time recovery enabled.

## Context

The data model is event-oriented, access patterns are known upfront, and traffic volume is low and spiky — long idle stretches punctuated by an import run.

## Rationale

DynamoDB provides:

- zero idle cost, which matters more than per-request price at this traffic shape
- predictable scaling
- strong integration with Lambda
- schema flexibility suitable for evolving insight structures
- atomic conditional writes, which [ADR-008](008-idempotency-via-deterministic-key.md) depends on for race-free deduplication

A relational database would introduce unnecessary operational and cost overhead: an always-on instance billed by the hour, for a workload that is idle most of the day.

## Consequences

- Access patterns must be designed explicitly, up front — see [ADR-012](012-single-table-design.md)
- Limited ad-hoc querying; there is no `WHERE` clause for a question nobody planned for
- Data modeling effort shifts to design time
- Point-in-time recovery is on, which costs a little and removes the "one bad migration script and it's gone" failure mode
- On-demand billing means a runaway import loop translates directly into spend rather than into throttling; the cost ceiling is behavioral, not configured
