# Insight Processing Platform

## One-sentence summary

A backend system that ingests Readwise webhook events, processes highlights asynchronously, enriches them with LLM-based analysis, and stores structured insights reliably and cost-efficiently.

---

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

---

## Architecture (high level)

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
  - optional LLM enrichment
  - persistence
      │
      ▼
DynamoDB
```

**Failure behavior**
- transient errors → retries
- permanent errors → DLQ
- LLM failure ≠ system failure

---

## Key design decisions (summary)

- **AWS managed primitives** for reliability and transparent cost modeling
- **API Gateway + Lambda + SQS** for decoupling, retries, and backpressure
- **Single core service** to avoid premature microservice complexity
- **DynamoDB (On-Demand)** for event-driven access patterns and zero idle cost
- **No Kubernetes** — control-plane cost and operational overhead are unjustified at this scale

All decisions are intentional and documented in ADRs.

---

## Cost philosophy

Costs are designed to be **visible and boring**.

At expected load (hundreds of events per month):

> **~8 € / month**

Rough order of magnitude:
- AWS infrastructure: ~5–10 €
- LLM usage: explicitly capped via token limits and alarms

The primary cost risk is **uncontrolled tokens**, not AWS.

---

## What this project demonstrates

- Event-driven system design
- Idempotent ingestion and retry safety
- Explicit failure paths (DLQ, fallback logic)
- Cost-aware and failure-tolerant LLM usage
- Operational thinking (logs, metrics, alarms)

This project is optimized for **system design signal**, not feature breadth.

---

## Explicit non-goals

- No Kubernetes
- No microservice sprawl
- No frontend focus
- No AI demo hype

Constraints are part of the design.

---

## Further documentation

- Architectural decisions: [`docs/adr.md`](docs/adr.md)
- Setup & development: [`docs/setup.md`](docs/setup.md)
