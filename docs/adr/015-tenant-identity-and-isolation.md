---
id: ADR-015
title: Tenant Identity and Isolation
status: Accepted
date: 2026-07-22
related: [ADR-012, ADR-016]
---

# ADR-015: Tenant Identity and Isolation

## Decision

Model tenancy in the data from the start, and derive the caller's tenant from an authenticated token rather than from the request.

- **Storage**: every item's partition key is `TENANT#<tenantID>` ([ADR-012](012-single-table-design.md)).
- **REST**: a Gin middleware validates a Cognito **ID token** against the user pool's JWKS and reads the tenant from the `custom:tenant_id` claim. Handlers read that value from the Gin context; the `tenantID` path parameter is untrusted and unused.
- **Ingest edge**: webhook and poll functions resolve a single tenant from the `DEFAULT_TENANT_ID` environment variable.

## Context

The system is single-user today, but insights are personal data and a data model without a tenant dimension is expensive to retrofit — every key, every query, and every stored item would have to change.

Authentication arrived later than the data model, and the two ingest paths have no user session to derive identity from: a Readwise webhook is a machine calling in, and a scheduled poll has no caller at all.

## Rationale

**Tenant in the key, not in a filter.** Isolation enforced by the partition key cannot be forgotten at a call site; a missing `WHERE tenant_id = ...` is a class of bug the key scheme makes unrepresentable.

**ID tokens, not access tokens.** Cognito only includes custom attributes on ID tokens, so `custom:tenant_id` forces that choice. The validator checks it explicitly (`token_use == "id"`), pins the issuer and audience, requires RS256 and an expiry, and rejects a token with a blank claim. This is unusual enough to be worth stating: an access token, which is the more conventional choice for API authorization, would authenticate the caller and carry no tenant.

**Claim, not path.** Taking the tenant from `/tenants/:tenantID/...` would let any authenticated user read any tenant by editing a URL. The claim is signed; the path is a suggestion.

**Env var at the ingest edge.** A webhook cannot present a token this system issued. Rather than invent a mapping from source account to tenant before there are two tenants, each ingest function is configured with the one tenant it serves.

## Consequences

- The read and write API is genuinely multi-tenant and enforces isolation per request.
- **The ingest edge is not.** Every webhook delivery and every poll is attributed to one configured tenant. Multi-tenant ingest needs a real mapping from source credential to tenant — a change to the ingest functions and their configuration, but not to the data model, which is the point of having paid for the tenant dimension early.
- JWKS is fetched once at startup and refreshed in the background for the process lifetime, so key rotation needs no redeploy — but a JWKS fetch failure at cold start fails the function's construction outright.
- Tenant provisioning is manual: creating a user means setting `custom:tenant_id` by hand. Acceptable at one tenant; the first real onboarding needs a story.
