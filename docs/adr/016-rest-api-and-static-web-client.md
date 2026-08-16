---
id: ADR-016
title: REST API and Static Web Client
status: Accepted
date: 2026-06-03
related: [ADR-002, ADR-007, ADR-015]
---

# ADR-016: REST API and Static Web Client

## Decision

Expose a versioned REST API (`/v1`) built with Gin, running as a Lambda behind API Gateway. Serve the browser client as a static React SPA from a private S3 bucket fronted by CloudFront with Origin Access Control.

```
GET  /v1/insights          list, optionally ?tag=
POST /v1/insights          manual create (synchronous — see ADR-007)
GET  /v1/tags              tag summaries
POST /v1/readwise/import   bulk import (enqueues)
POST /v1/raindrop/import   bulk import (enqueues)
```

Every route sits behind the Cognito middleware from [ADR-015](015-tenant-identity-and-isolation.md).

## Context

The pipeline had no read path: insights went in and were only observable in DynamoDB. A demonstrable system needs a way to look at what it produced, and a way to trigger an import without a CLI.

## Rationale

**REST over GraphQL.** Five endpoints, one consumer, no client-driven query shaping needed. GraphQL would add a schema layer and a resolver runtime to solve a problem this API does not have.

**Gin.** A router and middleware chain, which is what the auth boundary needs; the handlers below it are thin mappers into the same application services the worker uses ([ADR-005](005-hexagonal-architecture.md)).

**One Lambda for the whole API.** The router is constructed once and adapted to API Gateway, rather than deploying a function per route. Fewer cold starts, one IAM role, one deployment artifact.

**Static SPA, no server rendering.** The client is a build artifact — React and Vite, no state library, no CSS framework. It talks to the API from the browser with a Cognito ID token. Nothing about it needs a server, so it does not get one.

**Private bucket, CloudFront OAC.** The bucket is never public; only the distribution can read it. This costs one extra Terraform concept and removes the most common S3 hosting mistake outright.

## Consequences

- CORS is configured in the router only when origins are supplied, because API Gateway handles it in AWS and the Vite dev server needs it locally. Two environments, one code path, one conditional.
- The web client is explicitly a demonstration surface, not a product UI: no design system, no offline story, no error-state polish.
- API versioning exists as a path prefix from day one, so a breaking change has somewhere to go.
- A browser holding an ID token means token handling lives in client code; the SPA is only as safe as its token storage, and it is a demo.
- Adding an endpoint touches the router, a handler, and its DTO/mapper — deliberate friction that keeps wire shapes out of the domain.
