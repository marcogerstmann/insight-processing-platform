# Architecture Decision Records

This document captures the **core architectural decisions*- of the Insight Processing Platform.

The goal is not exhaustiveness, but **defensibility**: each decision is intentional, constrained, and reversible only with a clear trade-off.

---

## ADR-001: Cloud Platform - AWS

### Decision

Use AWS as the cloud provider.

## Context

The system requires managed primitives for:

- event ingestion
- asynchronous processing
- retry handling
- cost visibility
- minimal operational overhead

### Rationale

AWS provides mature, well-understood building blocks (Lambda, SQS, DynamoDB) that map directly to the system's needs. The platform enables fine-grained cost control and avoids maintaining undifferentiated infrastructure.

### Consequences

- Strong alignment with serverless and event-driven patterns
- Vendor lock-in is accepted as a deliberate trade-off
- Architecture favors managed services over custom runtime control

---

## ADR-002: Ingress & Decoupling - API Gateway + Lambda + SQS

### Decision

Use API Gateway and a lightweight Ingress Lambda to receive events, and SQS to decouple ingestion from processing.

### Context

Upstream events (e.g. Readwise webhooks) are external, bursty, and outside system control. Downstream processing includes potentially slow or failing dependencies (LLMs).

### Rationale

SQS provides:

- buffering during traffic spikes
- controlled retries
- backpressure when downstream systems are slow
- clear failure semantics via DLQ

This prevents slow LLM calls from blocking ingestion and keeps the system responsive under load.

### Consequences

- Increased latency compared to synchronous processing
- Clear separation between I/O concerns and domain logic
- Failure handling becomes explicit instead of implicit

---

## ADR-003: Compute Model - Serverless First

### Decision

Use AWS Lambda for ingress and for the core processing worker (via container image).

### Context

The system has low to moderate traffic, unpredictable load, and no strict latency requirements for background processing.

### Rationale

Lambda minimizes idle cost, removes server management, and fits naturally with event-driven workflows. Container images allow use of Go with full dependency control while retaining serverless benefits.

### Consequences

- Cold starts are accepted
- Execution time limits shape processing logic
- Horizontal scaling is automatic and managed

---

## ADR-004: Data Store - DynamoDB (On-Demand)

### Decision

Use DynamoDB with on-demand capacity for persistence.

### Context

The data model is event-oriented, access patterns are known upfront, and traffic volume is low.

### Rationale

DynamoDB provides:

- zero idle cost
- predictable scaling
- strong integration with Lambda
- schema flexibility suitable for evolving insight structures

A relational database would introduce unnecessary operational and cost overhead.

### Consequences

- Access patterns must be designed explicitly
- Limited ad-hoc querying
- Data modeling effort shifts to design time

---

## ADR-005: Service Topology - Single Core Service

### Decision

Implement all domain logic in a single core processing service.

### Context

The system's complexity lies in correctness, cost control, and failure handling - not in organizational scale.

### Rationale

A single service:

- reduces cognitive overhead
- simplifies deployment and observability
- avoids premature microservice boundaries

Decomposition can be revisited if real scaling pressures emerge.

### Consequences

- Clear ownership of domain logic
- Fewer moving parts
- Scaling is vertical and event-driven, not organizational

---

## ADR-006: Container Orchestration - No Kubernetes

### Decision

Do not use Kubernetes.

### Context

Traffic volume is low, system size is small, and operational simplicity is a priority.

### Rationale

Kubernetes would introduce:

- control-plane cost
- operational complexity
- additional failure modes

These costs are unjustified for the problem domain. Lambda and managed services provide sufficient scalability and reliability.

### Consequences

- Less flexibility in runtime customization
- Stronger coupling to AWS primitives
- Lower operational burden

---

## ADR-007: LLM Usage - Controlled Dependency

### Decision

Treat LLMs as an optional, failure-tolerant enrichment step.

### Context

LLMs are powerful but:

- slow
- probabilistic
- cost-sensitive
- externally operated

### Rationale

The system must remain correct and operational even if LLMs fail or are unavailable. Hard limits on tokens, timeouts, and retries ensure predictable cost and behavior.

### Consequences

- AI enrichment is best-effort
- Fallback processing is mandatory
- Cost overruns are prevented by design
