---
id: ADR-003
title: No Kubernetes
status: Accepted
date: 2026-01-04
related: [ADR-001, ADR-002]
---

# ADR-003: No Kubernetes

## Decision

Do not use Kubernetes.

## Context

Traffic volume is low, system size is small, and operational simplicity is a priority.

## Rationale

Kubernetes would introduce:

- control-plane cost
- operational complexity
- additional failure modes

These costs are unjustified for the problem domain. Lambda and managed services provide sufficient scalability and reliability.

## Consequences

- Less flexibility in runtime customization
- Stronger coupling to AWS primitives
- Lower operational burden
- Long-running or stateful work has no home; anything exceeding Lambda's execution ceiling would force this decision to be revisited rather than worked around
