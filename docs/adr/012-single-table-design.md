---
id: ADR-012
title: Single-Table Design with a Sparse Tag Index
status: Accepted
date: 2026-08-03
related: [ADR-008, ADR-011, ADR-015]
---

# ADR-012: Single-Table Design with a Sparse Tag Index

## Decision

Keep every item in one table, partitioned by tenant, with the item type encoded in the sort key:

| Item | `pk` | `sk` | `gsi1pk` / `gsi1sk` |
| --- | --- | --- | --- |
| Insight | `TENANT#<tenantID>` | `INSIGHT#<insightID>` | *(absent)* |
| Tag membership | `TENANT#<tenantID>` | `TAG#<tag>#INSIGHT#<insightID>` | `TENANT#<tenantID>` / `TAG#<tag>#...` |

A single GSI (`gsi1`) indexes tag membership items only. Because insight items never carry `gsi1pk`/`gsi1sk`, they are absent from the index entirely — the index is sparse by construction.

## Context

Two access patterns matter: list a tenant's insights, and list a tenant's insights carrying a given tag. Tags are produced by LLM enrichment ([ADR-013](013-llm-as-optional-enrichment.md)), arrive after the insight is first written, and change when an insight is re-enriched.

## Rationale

**One partition per tenant** means every query is a `Query` on a known `pk` with a `begins_with` on the sort key — never a `Scan`. It also makes tenant isolation a property of the key, not of a filter that could be forgotten ([ADR-015](015-tenant-identity-and-isolation.md)).

**Membership as separate items** rather than a list attribute on the insight: a tag list attribute cannot be queried without scanning, and updating it races with concurrent enrichment. Discrete membership items make "insights with tag X" a range query.

**A sparse GSI** is what keeps that cheap. Only membership items are projected, so the index holds tag rows and nothing else — no filtering out insight items at read time, and no write cost on the far more numerous plain insights. `ListTags` reads the index directly and never touches the full insight items.

Re-enrichment reconciles memberships rather than rewriting them, so a re-run does not reset `created_at`/`highlighted_at` or leave duplicate rows.

## Consequences

- A tag query is two round trips: resolve insight IDs from the index, then batch-fetch the items. Accepted — the alternative is duplicating full insight bodies into the index.
- Tag membership rows are derived data. If enrichment and reconciliation disagree, the memberships are wrong and there is no background repair job; correctness rests on the reconcile path being right.
- The GSI is behind a Terraform flag (`enable_tag_gsi`), so the table can be provisioned without it. The adapter's `tagIndexName` constant must match the Terraform name — a coupling across two languages that nothing enforces.
- Adding a third access pattern likely means another sort-key prefix rather than another table, and the key scheme should stay documented here as it grows.
- Ranking uses `highlighted_at` (the source system's timestamp), deliberately stored separately from the `created_at`/`updated_at` audit fields so that ingest order never distorts relevance.
