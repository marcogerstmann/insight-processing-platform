---
id: ADR-002
title: Serverless-First Compute
status: Accepted
date: 2026-01-04
related: [ADR-001, ADR-003, ADR-006, ADR-007]
---

# ADR-002: Serverless-First Compute

## Decision

Run every unit of compute as an AWS Lambda function. Package the processing worker as a container image and the edge functions (Readwise webhook, Raindrop poll, REST API) as zip archives.

## Context

The system has low to moderate traffic, unpredictable load, and no strict latency requirements for background processing. There are four deployed functions:

| Function | Trigger | Packaging |
| --- | --- | --- |
| `readwise` | API Gateway (webhook) | zip |
| `raindrop-poll` | EventBridge Scheduler | zip |
| `rest` | API Gateway (HTTP API) | zip |
| `worker` | SQS event source mapping | container image |

## Rationale

Lambda minimizes idle cost, removes server management, and fits naturally with event-driven workflows.

The worker ships as a container image because it carries the heaviest dependency set (AWS SDK, OpenAI SDK) and benefits from full control over the build; the edge functions stay as zips, which deploy faster and cold-start smaller for the thin request-mapping work they do. Mixing both packaging modes is deliberate — `terraform/modules/lambda-zip` and `terraform/modules/lambda-image` exist side by side so each function pays only for what it needs.

## Consequences

- Cold starts are accepted
- Execution time limits shape processing logic; the worker runs with a 30s timeout and 256 MB, which bounds how much LLM latency a single message can absorb ([ADR-013](013-llm-as-optional-enrichment.md) caps the call at 30s to match)
- Horizontal scaling is automatic and managed
- Two packaging paths mean two build lanes in CI and two Terraform modules to maintain — accepted for the cold-start and image-size trade-off it buys
