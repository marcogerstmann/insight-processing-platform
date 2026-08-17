---
id: ADR-006
title: Manual Dependency Injection and Paired Lambda/Local Entrypoints
status: Accepted
date: 2026-01-25
related: [ADR-002, ADR-005]
---

# ADR-006: Manual Dependency Injection and Paired Lambda/Local Entrypoints

## Decision

Wire dependencies by hand in `main()`. No DI container, no code generation. Every deployed function ships as a pair of entrypoints under `cmd/`:

```
cmd/worker-lambda/   real adapters (DynamoDB, SQS, EventBridge, OpenAI, SSM)
cmd/worker-local/    in-memory adapters, no AWS credentials required
```

The same for `readwise`, `raindrop-poll`, and `rest`. Both members of a pair construct the *same* handler from `internal/adapters/inbound/`; only the outbound adapters differ.

## Context

Iterating against deployed Lambdas is slow, and the alternative — LocalStack or a full docker-compose AWS emulation — is a second infrastructure stack to install, version, and debug when it diverges from real AWS.

## Rationale

[ADR-005](005-hexagonal-architecture.md) already forces every outbound dependency through an interface. Once that is true, a local runner is not a testing framework — it is a different `main()` that passes `internal/adapters/outbound/memory` implementations instead of AWS ones. The cost of the local lane is roughly one file per function, and it needs no emulator, no containers, and no credentials.

Manual DI follows from the same place. With four functions and a handful of ports, `NewService(repo, llm, events)` in `main()` is legible top to bottom and fails at compile time. A container would add a dependency and move the wiring errors to runtime.

## Consequences

- The local runner exercises real handler, mapper, and service code; only the edges are fake. Bugs in DynamoDB expressions or IAM policies are *not* caught locally and never will be.
- Each `main()` repeats a similar construction block. That duplication is accepted — it is the price of each entrypoint being readable without indirection, and it stays bounded as long as the function count does.
- Adding a port means touching both entrypoints of a pair, and the compiler says so.
- Secrets resolve through one indirection (`envutil.ResolveSecret`): an env var holding a literal value locally, or an `ssm:`-prefixed parameter path in AWS. One code path, two behaviors, no branching in `main()`.
