---
id: ADR-018
title: One Provider for All Model Capabilities
status: Accepted
date: 2026-08-17
related: [ADR-005, ADR-013, ADR-017]
---

# ADR-018: One Provider for All Model Capabilities

## Decision

Serve every model capability — LLM enrichment and text embeddings — from a single provider under a single API key. OpenAI is that provider: `gpt-5.6-luna` for enrichment, `text-embedding-3-small` at **512 dimensions** for embeddings.

## Context

The system arrived at two providers by accretion rather than by choice. Enrichment used Anthropic from the start ([ADR-013](013-llm-as-optional-enrichment.md)). When embeddings were added, Anthropic served none, so a second vendor — Voyage — came with them.

Two vendors meant two of everything downstream of the choice: two accounts, two SSM parameters, two resource-scoped IAM statements, two `ssm:`-prefixed env vars, two HTTP clients, two rotation paths. None of that duplication expressed a decision; it was the cost of an absent capability.

That absence no longer holds. One provider now covers both, and a third capability (a reranker, a vision model) would arrive as a model ID rather than as an infrastructure story.

## Rationale

**A gateway was the obvious alternative and was rejected.** OpenRouter fronts many vendors behind one OpenAI-compatible key and does serve embeddings. But its value is cross-vendor experimentation, which this system does not do — it makes one enrichment call and one embedding call, both settled. Against that it adds a third-party dependency in front of the pipeline, a network hop, and a 5.5% fee on credit top-ups. Committing to one vendor's models makes the gateway pure overhead.

**The embedding model had to change regardless.** Voyage is not reachable through any consolidation route, so this was never a choice between "keep the vectors" and "change vendor" — only between which new model. That removed the main argument for staying.

**512 dimensions is the decision inside the decision.** `text-embedding-3-*` accept a `dimensions` parameter; the models are trained so a shortened vector keeps its concept-representing properties. Vectors here are compared by brute force with no index and stored as DynamoDB `Decimal` lists, so width sets item size, read-capacity cost and per-comparison cost simultaneously. 512 instead of the 1536 default is roughly a 3× reduction in all three. Voyage's fixed 1024 offered no such control; the knob is a genuine gain from the move, not a consolation for it.

**The ports made this cheap, which is what they are for.** `ports.EnrichmentClient` and the Python `EmbeddingClient` protocol meant the change was one adapter per service. Nothing in either application layer knows which vendor answers, and the enrichment bounds ADR-013 specifies — 512 output tokens, 30s, 3 retries, degrade-not-fail — were re-expressed against the new SDK without weakening.

## Consequences

- **Single-vendor concentration is now real.** An OpenAI outage stops enrichment *and* embeddings, where those previously failed independently. The blast radius is bounded by ADR-013: insights are still written and the pipeline still runs; tags and vectors stop appearing until the provider returns.
- **No vector migration was needed, by luck rather than design.** The embeddings table was empty when this landed — the Voyage parameter had never been provisioned in dev, so the adapter shipped in IPP-97 had never successfully written a vector. Had the corpus been populated, every stored `voyage-3` vector would have been invalid the moment the model changed, and the switch would have required a full re-embed first.
- Because that failure is silent by nature — cosine similarity across two vector spaces returns a plausible number and no error — `domain.embedding.cosine_similarity` raises `IncomparableEmbeddings` when `model` or `dimension` differ. `model`/`dimension` were already stored per item ([ADR-017](017-idiomatic-python-in-the-ai-service.md)-era work under IPP-97), which is the only reason a mixed state would be detectable at all. The next model change will not have an empty table to fall back on.
- The enrichment adapter now uses **structured outputs** (`strict: true`) instead of a forced tool call. Same guarantee of shape, one response field instead of a tool definition plus a scan of the content blocks.
- Enrichment output is model-dependent and the model changed, so the two were run head to head on 18 representative highlights with the same prompt and the same output shape. Tag counts, altitude spread and the absence of wording-bound one-offs matched; the new model showed a mild tendency to spend one of its five tags on a generic term (`focus`, `action-items`) where the old one stayed specific. Judged not a regression, and recorded here rather than smoothed over.
- The API key is a SecureString created **out of band**, not an `aws_ssm_parameter` resource — a managed one would put the key in Terraform state. Terraform grants read access to it; it never holds it.
- **Reversal trigger:** wanting a model OpenAI does not serve. At that point the choice is a second key again or a gateway, and the ports make either one adapter per service. Reconsider then, not before.
