"""Ports the application depends on.

Go declares one `interface` per file under internal/ports; here every port
is a `typing.Protocol`, collected in this one module — there are few enough
of them that one file per port would be Go ceremony, not Python idiom.

`Protocol`, not `abc.ABC`: an adapter satisfies a Protocol structurally,
without importing it, which is what keeps the dependency direction
(adapters depend on nothing in this service) intact. See ADR-017.
"""

from __future__ import annotations

from typing import Protocol

from ipp_ai.domain.action import Action
from ipp_ai.domain.embedding import Embedding
from ipp_ai.domain.insight import Insight
from ipp_ai.domain.plan_context import ContextEdge
from ipp_ai.domain.relationship import RelatedInsight, RelationJudgement, Relationship


class SecretProvider(Protocol):
    """Resolves a secret by name. Mirrors internal/ports.SecretProvider."""

    def get(self, name: str) -> str: ...


class InsightReader(Protocol):
    """Read-only access to the insights table.

    Deliberately not internal/ports.InsightRepository translated wholesale:
    this service never writes, so there is no CreateIfAbsent/Update here —
    see services/ai/README.md for why writes go through the Go REST API
    instead. get_by_id has no Go counterpart either; the Go repository never
    needed one; this service does, to load a single insight for an agent.
    """

    def get_by_id(self, tenant_id: str, insight_id: str) -> Insight | None: ...
    def list_by_tenant(self, tenant_id: str) -> list[Insight]: ...
    def list_by_tag(self, tenant_id: str, tag: str) -> list[Insight]: ...


class RelationshipReader(Protocol):
    """Read-only access to an insight's relationship edges (REL 6/IPP-102),
    the same query GET /v1/tenants/:tenantID/insights/:insightID/relationships
    runs — but read directly from the shared table rather than through the Go
    API, same as InsightReader (services/ai/README.md's boundary rule is
    about writes, not reads).
    """

    def list_by_insight(self, tenant_id: str, insight_id: str) -> list[RelatedInsight]: ...


class EmbeddingClient(Protocol):
    """Turns text into a vector. Mirrors internal/ports.EnrichmentClient's
    shape (one bounded call in, one result out) but the result is a
    fixed-size vector rather than tags; `model`/`dimension` are exposed so
    a caller can stamp them on the stored Embedding (per IPP-97) without
    the port hard-coding one provider's identifiers.
    """

    model: str
    dimension: int

    def embed(self, text: str) -> tuple[float, ...]: ...


class EmbeddingWriter(Protocol):
    """Upserts an insight's embedding into the AI service's own table.

    Unlike InsightReader this is a write — see adapters/outbound/embedding_store.py
    for why this table, unlike the shared insights table, is this service's
    to write. Idempotent: re-processing the same event overwrites in place.
    """

    def put(self, embedding: Embedding) -> None: ...


class EmbeddingReader(Protocol):
    """Read-only access to this service's own embeddings table — REL 2's
    candidate pool. Same table EmbeddingWriter.put writes to; kept as a
    separate Protocol rather than folded into EmbeddingWriter because a
    caller that only lists (REL 2) shouldn't have to depend on put too.
    """

    def list_by_tenant(self, tenant_id: str) -> list[Embedding]: ...


class RelationLabeler(Protocol):
    """Labels a candidate pair via one bounded LLM call — the read side's
    counterpart to internal/adapters/outbound/openai.Client.Enrich's shape
    (one call in, one structured result out), returning a judgement about
    two insights instead of tags for one. Always returns a judgement or
    raises; the confidence threshold that turns a judgement into a stored
    Relationship is a policy decision for the caller
    (application/relationship.py), not this port.
    """

    def label(self, from_text: str, to_text: str) -> RelationJudgement: ...


class ActionGenerator(Protocol):
    """Drafts 3-5 candidate actions from a focus sentence and a bounded
    context — the read side's counterpart to RelationLabeler.label's shape
    (one bounded call in, one structured result out). Each draft's
    `supporting_insight_ids` are unverified model output; checking them
    against the ids `insights` actually contains is
    application/action_generation.py's job, not this port's, same split
    RelationLabeler/label_relationships draws for the confidence threshold.
    """

    def generate(
        self, focus_sentence: str, insights: list[Insight], edges: tuple[ContextEdge, ...]
    ) -> list[Action]: ...


class RelationshipWriter(Protocol):
    """Persists a discovered relationship through the Go REST API (REL 4).

    Unlike EmbeddingWriter, this write leaves the AI service's own AWS
    account: a Relationship is domain data, so it goes through the Go API
    rather than DynamoDB directly (services/ai/README.md's boundary rule).
    """

    def put(self, relationship: Relationship) -> None: ...


class PlanResultWriter(Protocol):
    """Persists a weekly plan's outcome through the Go REST API (PLAN 4).

    Same boundary rule as RelationshipWriter: a WeeklyPlan's result is
    domain data, so it leaves this service's own AWS account through the Go
    API rather than being written to DynamoDB directly.
    """

    def set_ready(self, tenant_id: str, plan_id: str, actions: list[Action]) -> None: ...
    def set_failed(self, tenant_id: str, plan_id: str, reason: str) -> None: ...


class DlqPublisher(Protocol):
    """Forwards a failed record's raw body to a dead-letter queue, tagged
    with the failure reason. Mirrors internal/ports.DLQPublisher — narrower,
    since nothing here needs to forward SQS message attributes (this
    service's only inbound record shape is an EventBridge envelope, which
    carries none worth preserving).
    """

    def send(self, body: str, reason: str) -> None: ...
