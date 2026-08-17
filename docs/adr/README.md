# Architecture Decision Records

This folder captures the **core architectural decisions** of the Insight Processing Platform.

The goal is not exhaustiveness, but **defensibility**: each decision is intentional, constrained, and reversible only with a clear trade-off.

## Conventions

One file per ADR, `NNN-slug.md`, with YAML frontmatter (`id`, `title`, `status`, `date`, `related`) and the same four sections: **Decision**, **Context**, **Rationale**, **Consequences**.

Numbers group decisions by theme so the set reads as one argument, from platform outward to interfaces. They are **not** chronological — `date` is, and it records when the decision was actually taken, not when it was written down.

**Numbers are permanent.** The 001–016 set was renumbered once, on 2026-08-16, when the single `docs/adr.md` was split into this folder. That was the last time. A number is a citable address; new decisions append at the next free number, wherever they land thematically.

Which means there are two different kinds of change, and they are not interchangeable:

- **The record was wrong** — the ADR described the system inaccurately, or a consequence went unstated. Edit the file in place. `date` does not move; the decision did not change, only the description of it.
- **The decision changed** — the old ADR gains `status: Superseded` and a `superseded-by: ADR-0NN` field, and the new decision is written as a new ADR that names what it replaces and why. The superseded file keeps its number, its text, and its row in the index. Do not rewrite it to argue against itself; its being wrong later is the record.

## Foundation

| ADR | Decision | Date |
| --- | --- | --- |
| [ADR-001](001-aws-as-cloud-platform.md) | AWS as Cloud Platform | 2026-01-04 |
| [ADR-002](002-serverless-first-compute.md) | Serverless-First Compute | 2026-01-04 |
| [ADR-003](003-no-kubernetes.md) | No Kubernetes | 2026-01-04 |
| [ADR-004](004-terraform-and-oidc-federated-ci.md) | Terraform and OIDC-Federated CI | 2026-06-02 |

## Code structure

| ADR | Decision | Date |
| --- | --- | --- |
| [ADR-005](005-hexagonal-architecture.md) | Hexagonal Architecture (Ports and Adapters) | 2026-01-16 |
| [ADR-006](006-manual-di-and-paired-entrypoints.md) | Manual DI and Paired Lambda/Local Entrypoints | 2026-01-25 |
| [ADR-017](017-idiomatic-python-in-the-ai-service.md) | Idiomatic Python in the AI Service | 2026-08-16 |

## Ingest pipeline

| ADR | Decision | Date |
| --- | --- | --- |
| [ADR-007](007-asynchronous-ingest-via-sqs.md) | Asynchronous Ingest via SQS | 2026-01-04 |
| [ADR-008](008-idempotency-via-deterministic-key.md) | Idempotency via a Deterministic Event Key | 2026-01-18 |
| [ADR-009](009-error-taxonomy-and-dlq-routing.md) | Error Taxonomy and Explicit DLQ Routing | 2026-06-04 |
| [ADR-010](010-multi-source-ingestion.md) | Multi-Source Ingestion | 2026-08-16 |

## Persistence

| ADR | Decision | Date |
| --- | --- | --- |
| [ADR-011](011-dynamodb-as-data-store.md) | DynamoDB as Data Store | 2026-01-04 |
| [ADR-012](012-single-table-design.md) | Single-Table Design with a Sparse Tag Index | 2026-08-03 |

## Enrichment and events

| ADR | Decision | Date |
| --- | --- | --- |
| [ADR-013](013-llm-as-optional-enrichment.md) | LLM as Optional Enrichment | 2026-01-04 |
| [ADR-014](014-domain-events-on-eventbridge.md) | Domain Events on EventBridge | 2026-08-13 |
| [ADR-018](018-one-provider-for-model-capabilities.md) | One Provider for All Model Capabilities | 2026-08-17 |

## Access

| ADR | Decision | Date |
| --- | --- | --- |
| [ADR-015](015-tenant-identity-and-isolation.md) | Tenant Identity and Isolation | 2026-07-22 |
| [ADR-016](016-rest-api-and-static-web-client.md) | REST API and Static Web Client | 2026-06-03 |
