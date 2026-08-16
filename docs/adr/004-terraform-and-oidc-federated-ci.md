---
id: ADR-004
title: Terraform and OIDC-Federated CI
status: Accepted
date: 2026-06-02
related: [ADR-001, ADR-002]
---

# ADR-004: Terraform and OIDC-Federated CI

## Decision

Define all infrastructure in Terraform, split into reusable `terraform/modules/` and a per-environment root in `terraform/envs/<env>/`. Deploy from GitHub Actions using an IAM role assumed via GitHub's OIDC provider — no long-lived AWS access keys anywhere.

## Context

The system spans a dozen AWS resource types across Lambda, SQS, DynamoDB, API Gateway, EventBridge, Cognito, S3, and CloudFront. Wiring them by hand in the console would make the "cost visibility and low operational overhead" claim of [ADR-001](001-aws-as-cloud-platform.md) unverifiable — nothing would record why a resource exists or what it costs.

Only one environment (`dev`) is deployed today.

## Rationale

**Modules plus a thin env root.** Modules hold the shape of a resource (`lambda-zip`, `lambda-image`, `sqs`, `dynamodb`, `api-gateway`, `eventbridge`, `iam`); the env root holds the wiring and the names. Adding a second environment means a new root directory, not a rewrite.

**OIDC over stored keys.** `terraform/envs/dev/github-actions.tf` federates `token.actions.githubusercontent.com` and constrains the trust policy by `aud` and `sub`, so CI receives short-lived credentials scoped to this repository. Storing an access key pair in repository secrets would be simpler to set up and strictly worse: it would be a standing credential with no natural expiry.

## Consequences

- Every resource is reviewable in a pull request, and drift is visible via `terraform plan`
- The IAM policy granted to CI is broad enough to manage the stack it deploys, including IAM itself — a real blast radius, accepted because the trust policy pins it to one repository
- One environment exists; the module/env split is a bet on a second one that has not been collected yet
- Terraform state lives in a remote backend (`backend.tf`), so CI and local runs share one source of truth
