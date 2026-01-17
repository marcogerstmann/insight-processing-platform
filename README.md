# Insight Processing Platform

## Project description

A system that ingests Readwise webhook events, processes highlights asynchronously, enriches them with LLM‑based analysis, and stores structured insights reliably and cost‑efficiently.

This system uses LLMs as a controlled dependency to transform unstructured inputs into actionable insights, improving knowledge work productivity while maintaining reliability and cost predictability.

---

### What this project is — and is not

This is not an AI product. It is an **event‑driven backend system** that uses LLMs as a controlled dependency.

LLMs are:

- interchangeable
- budget‑limited
- isolated from core system reliability

Readwise is:

- a data source
- an event generator
- treated purely as an event source

The value of the system lies in how events are **processed, enriched, and stored** - not in the integration itself.

The architecture is intentionally source-agnostic and remains valid if Readwise is replaced by another upstream provider.

The focus is system design, cost control, and operational clarity.

---

### High‑level architecture

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

Failure paths:

- transient errors → retries
- permanent errors → DLQ
- LLM failure ≠ system failure

---

### Core design decisions (summary)

- **AWS** for managed primitives and transparent cost modeling
- **Lambda + SQS** for decoupling, retries, and backpressure handling
- **One core service** to avoid premature microservice complexity
- **No Kubernetes** - unjustified control‑plane cost and operational overhead at this scale
- **DynamoDB (On‑Demand)** for event‑driven access patterns and zero idle cost

Each of these decisions is intentional and documented.

---

### Cost philosophy

TODO: Verify the costs

This system is designed to make costs visible and boring.

At current expected load (hundreds of events per month):

> ~8 € / month

Rough breakdown:

- AWS infrastructure: ~5–10 €
- LLM usage: explicitly capped via token limits and alarms

The main risk is not AWS - it is uncontrolled token usage.

---

### What this project demonstrates

- Event‑driven system design
- Idempotent ingestion and retry safety
- Explicit failure paths
- Cost‑aware use of LLMs
- Operational thinking (logs, metrics, alarms)

---

### Non‑goals

- No Kubernetes
- No microservice sprawl
- No frontend focus
- No AI hype demos

Constraints are part of the design.

## Project setup

### Environment variables

The following environment variables are used in the project:

| Variable                | Description                                                       |
|-------------------------|-------------------------------------------------------------------|
| READWISE_WEBHOOK_SECRET | Shared secret used to verify incoming Readwise webhook signatures |

### Local usage

#### Simulate ingest Lambda locally

Run the local webhook listener:

```bash
go run ./cmd/ingest-local/main.go
```

Send a test webhook event using the prepared test payloads in `./httpRequests`.
