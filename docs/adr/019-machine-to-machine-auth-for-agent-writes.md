---
id: ADR-019
title: Machine-to-Machine Auth for Agent Writes
status: Accepted
date: 2026-08-18
related: [ADR-015, ADR-017]
---

# ADR-019: Machine-to-Machine Auth for Agent Writes

## Decision

Give the AI service its own Cognito identity, distinct from any user, via the OAuth2 **client_credentials** grant against a second app client scoped to a single custom permission (`ipp/agent.write`). The REST API's Gin middleware accepts either a human's Cognito **ID token** or the AI service's Cognito **access token**, discriminates on the token's `token_use` claim, and exposes which kind authenticated so routes can require one or the other. A service token carries no tenant claim — routes it may call take the tenant as an explicit parameter instead of reading it from an authenticated claim the way user routes do.

## Context

[ADR-015](015-tenant-identity-and-isolation.md) settled how a **human** proves who they are and which tenant they act within: an ID token, validated for `token_use == "id"`, with the tenant read from `custom:tenant_id`. That decision explicitly did not need to consider a caller with no session and no tenant — every existing caller was either a logged-in user or an ingest edge function pinned to one tenant by configuration.

REL 4 ([IPP-100](../../services/ai)) is the first caller that is neither: the AI service's relationship-discovery agent needs to write through the REST API — never around it, into DynamoDB directly, or REL 2/REL 3's arithmetic and LLM judgement stay siloed in a service with no durable effect. "The Python service writes only through the Go API" has been true by convention since [ADR-017](017-idiomatic-python-in-the-ai-service.md); this ADR is what makes it true by construction. Without a caller identity, the API has no way to tell "the AI service, entitled to write relationships" from "an attacker who found the Lambda's URL" — the first inconvenient moment (a deadline, a debugging session) turns into a direct table write and the boundary quietly dies.

## Rationale

**A second app client on the same user pool, not a second user pool.** Cognito's client_credentials grant needs a resource server (to define a custom scope) and an app client configured for that flow — both scoped to a user pool, not global. Reusing the existing pool avoids a second JWKS endpoint, a second issuer to trust, and a second Terraform-managed identity surface for one new capability.

**Discriminate on `token_use`, not on a new header or a shared secret.** Cognito already stamps every token — ID or access — with `token_use`. Branching on a claim the token format already carries is smaller and harder to spoof than inventing a second authentication channel (an API key header, mTLS) that the existing JWKS-based validator would then have to special-case around.

**No `aud` claim on an access token — this is the trap the acceptance criteria named up front.** ID tokens carry the app client ID in `aud`; Cognito access tokens (a user's own, and the AI service's machine token alike) carry no `aud` at all — the client is identified by `client_id` instead. The validator's shared `jwt.Parse` call therefore cannot enforce a single audience with `jwt.WithAudience`; each token-type branch checks its own claim (`aud` for a user, `client_id` for a service) after parsing. API Gateway's HTTP API JWT authorizer makes the same substitution for Cognito tokens specifically, matching `audience` against `client_id` when `aud` is absent — the Terraform authorizer's `audience` list carries both app client IDs for that reason.

**No tenant on a machine token is a feature, enforced by omission, not a gap patched with a default.** A service principal's tenant is never inferred — `authenticateService` never reads `custom:tenant_id`, even if a token were crafted to carry one. Any agent-only route takes the tenant as an explicit parameter (mirroring how [ADR-015](015-tenant-identity-and-isolation.md)'s ingest edge takes it as configuration, for the same reason: no session to derive it from). A handler that forgets this and reads the user-tenant context key for a service caller gets Gin's not-set zero value, not another tenant's data.

**Route authorization is a second, explicit middleware — not a side effect of which claim happened to parse.** `RequireUser()` and `RequireScope(scope)` run after the base `Middleware()` and enforce which principal type a route accepts. Every route wraps exactly one of them: today, every real route is user-only (`RequireUser()`); REL 4's future relationship-write route will be the first to use `RequireScope("ipp/agent.write")`. Applying `RequireUser()` at the route rather than the shared `/v1` group is deliberate — a group-level blanket would 403 the AI service's own token before a scope-specific check on a sibling route in the same group ever ran.

**The client secret is Terraform-managed, unlike the OpenAI key.** [ADR-018](018-one-provider-for-model-capabilities.md) keeps the OpenAI key **out of** Terraform state because it is hand-typed and Terraform would be the first place it ever appears in the system. The agent client's secret is different: Cognito generates it as an attribute of a Terraform-managed resource, so it is already in that resource's state the moment it's created. Wrapping it in a managed `aws_ssm_parameter` exposes nothing a plan/apply didn't already see, and gets it into the same `ssm:`-prefixed convention (`internal/envutil.ResolveSecret`, `services/ai`'s `application/secrets.py`) every other secret in this system follows.

**The Python token client caches, because client_credentials is a token-issuing call, not free.** Fetching a fresh token per outbound request would multiply Cognito calls by every write the agent makes for no benefit — the grant has no per-request state to refresh. `CognitoServiceTokenClient` caches until shortly before the token's own `expires_in`, matching the acceptance criteria's explicit requirement and the same bounded-retry discipline as `adapters/outbound/openai.py`.

## Consequences

- The REST API now trusts two identities instead of one: a compromised agent-client secret can write anything `ipp/agent.write` permits, across every tenant a caller names — the scope is not itself tenant-scoped, because there is exactly one machine caller today. Multi-tenant agents would need a per-tenant scope or claim, not built here because there is only one tenant to serve.
- Every existing route gained one line of middleware (`auth.RequireUser()`) with no behavior change for a human caller — a user token still authenticates and reaches the handler exactly as before.
- `CognitoValidator.authenticate` no longer returns a bare tenant string; it returns a `Principal{Type, TenantID, Scopes}`. `TenantIDKey` is unchanged for callers that only ever handle user routes (the three existing handlers), but any new handler on a mixed route must check `PrincipalTypeKey` before trusting `TenantIDKey` is populated.
- This ADR builds the mechanism only — no route uses `RequireScope` yet. REL 4 (IPP-100) is the first real consumer, on both sides: the Go `POST .../relationships` handler guarded by `RequireScope("ipp/agent.write")`, and the Python write adapter that calls `CognitoServiceTokenClient.token()` for its `Authorization` header.
- **Reversal trigger:** a second machine caller with different needs (a different scope, a different tenant scope) is the point to reconsider a per-tenant or per-caller claim scheme instead of one shared scope on one shared client.
