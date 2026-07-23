# Insight Processing Platform

An event-driven backend system that ingests webhook events, processes them asynchronously, enriches them with LLM-based
analysis, and stores structured insights using idempotent, reliable pipelines.

**Built with:** Go, AWS Lambda, SQS, DynamoDB, API Gateway, Cognito, Terraform, LLM (Anthropic Claude)

## What this project is - and is not

This is **not** an AI product.

It is an **event-driven backend system** that uses LLMs as a **controlled, optional dependency**.

**LLMs**

- interchangeable
- strictly budget-limited
- isolated from core system reliability

**Readwise**

- data source
- event generator
- not the business model

The value lies in **how events are processed, enriched, and operated**, not in the integration itself.

The architecture is source-agnostic and remains valid if Readwise is replaced.

## High-level architecture overview

```
Readwise Webhook
      │
      ▼
API Gateway
      │
Ingest Lambda
  - validate
  - normalize
  - generate idempotency key
      │
      ▼
SQS Queue
  - buffering
  - retry control
  - backpressure
      │
      ▼
Core Processing Service (Go)
  - domain logic
  - idempotent persistence
      │
      ├─────────────────────────┐
      │                         ▼
      │                Anthropic Claude
      │                  - enrich insight
      │                  - timeout + retry + token cap
      │                  - soft-fail: LLM down != system down
      │                         │
      ◄─────────────────────────┘
      │
      ▼
DynamoDB
```

**Failure behavior**

- transient errors → retries
- permanent errors → DLQ
- LLM failure ≠ system failure

## Key design decisions (summary)

- **AWS managed primitives** for reliability and transparent cost modeling
- **API Gateway + Lambda + SQS** for decoupling, retries, and backpressure
- **Single core service** to avoid premature microservice complexity
- **DynamoDB (On-Demand)** for event-driven access patterns and zero idle cost
- **No Kubernetes**: control-plane cost and operational overhead are unjustified at this scale

All decisions are intentional and documented in ADRs.

## What this project demonstrates

- Event-driven system design with explicit decoupling at every layer
- Idempotent ingestion and safe at-least-once delivery handling
- Explicit failure taxonomy: DLQ for permanent errors, retries for transient, soft-fail for LLM
- LLM integration as a controlled dependency — timeout, retry-with-backoff, token cap, graceful degradation
- Hexagonal architecture: domain logic fully decoupled from AWS infrastructure via ports and adapters
- Operational thinking (structured logging, cost-aware design, alarms)

This project is optimized for **system design signal**, not feature breadth.

## Explicit non-goals

- No Kubernetes
- No microservice sprawl
- No frontend focus
- No AI demo hype

Constraints are part of the design.

## Further documentation

- Architectural decisions: [`docs/adr.md`](docs/adr.md)
- Setup & development: [`docs/setup.md`](docs/setup.md)
- Web UI (demo client): [`web/README.md`](web/README.md)
