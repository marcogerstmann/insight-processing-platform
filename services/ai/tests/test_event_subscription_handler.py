from __future__ import annotations

import json
from dataclasses import dataclass, field

import pytest

from ipp_ai.adapters.inbound.event_subscription import Handler
from ipp_ai.domain.embedding import Embedding
from ipp_ai.domain.insight import Insight
from ipp_ai.domain.relationship import RelatedInsight, RelationJudgement, Relationship, RelationType


@dataclass
class SpyDlq:
    sent: list[tuple[str, str]] = field(default_factory=list)
    send_error: Exception | None = None

    def send(self, body: str, reason: str) -> None:
        self.sent.append((body, reason))
        if self.send_error is not None:
            raise self.send_error


@dataclass
class FakeReader:
    insights: dict[tuple[str, str], Insight] = field(default_factory=dict)
    by_tag: dict[tuple[str, str], list[Insight]] = field(default_factory=dict)

    def get_by_id(self, tenant_id: str, insight_id: str) -> Insight | None:
        return self.insights.get((tenant_id, insight_id))

    def list_by_tag(self, tenant_id: str, tag: str) -> list[Insight]:
        return self.by_tag.get((tenant_id, tag), [])


@dataclass
class FakeRelationshipReader:
    edges: dict[tuple[str, str], list[RelatedInsight]] = field(default_factory=dict)

    def list_by_insight(self, tenant_id: str, insight_id: str) -> list[RelatedInsight]:
        return self.edges.get((tenant_id, insight_id), [])


@dataclass
class FakeEmbedder:
    model: str = "text-embedding-3-small"
    dimension: int = 3
    vector: tuple[float, ...] = (0.1, 0.2, 0.3)
    error: Exception | None = None

    def embed(self, text: str) -> tuple[float, ...]:
        if self.error is not None:
            raise self.error
        return self.vector


@dataclass
class SpyWriter:
    puts: list[object] = field(default_factory=list)

    def put(self, embedding: object) -> None:
        self.puts.append(embedding)


@dataclass
class FakeEmbeddingReader:
    embeddings: list[Embedding] = field(default_factory=list)

    def list_by_tenant(self, tenant_id: str) -> list[Embedding]:
        return [e for e in self.embeddings if e.tenant_id == tenant_id]


@dataclass
class FakeLabeler:
    judgement: RelationJudgement | Exception | None = None

    def label(self, from_text: str, to_text: str) -> RelationJudgement:
        if isinstance(self.judgement, Exception):
            raise self.judgement
        if self.judgement is None:
            raise AssertionError("label() called but no judgement was configured")
        return self.judgement


@dataclass
class SpyRelationshipWriter:
    puts: list[Relationship] = field(default_factory=list)

    def put(self, relationship: Relationship) -> None:
        self.puts.append(relationship)


def _insight(insight_id: str = "i1", tenant_id: str = "t1") -> Insight:
    from datetime import datetime

    # tz-aware, matching real Insight values: DynamoDbInsightReader parses
    # highlighted_at via datetime.fromisoformat on Go's "Z"-suffixed RFC3339,
    # which yields an aware datetime — plan_context.select_context (IPP-104)
    # subtracts this from an aware `now`, so a naive stand-in here would fail.
    return Insight(
        id=insight_id,
        tenant_id=tenant_id,
        source="readwise",
        text="hello world",
        notes="",
        highlighted_at=datetime.fromisoformat("2026-01-01T00:00:00+00:00"),
    )


def _envelope(
    *, event_type: str = "InsightEnriched", tenant_id: str = "t1", insight_id: str = "i1"
) -> str:
    return json.dumps(
        {
            "version": "0",
            "id": "evt-1",
            "detail-type": event_type,
            "source": "ipp.core",
            "account": "123456789012",
            "time": "2026-08-16T00:00:00Z",
            "region": "eu-central-1",
            "resources": [],
            "detail": {
                "event_id": "abc123",
                "event_type": event_type,
                "version": 1,
                "tenant_id": tenant_id,
                "occurred_at": "2026-08-16T00:00:00Z",
                "payload": {"insight_id": insight_id, "tags": ["work"]},
            },
        }
    )


def _weekly_plan_requested_envelope(
    *, tenant_id: str = "t1", plan_id: str = "p1", tag: str = "golang"
) -> str:
    return json.dumps(
        {
            "version": "0",
            "id": "evt-2",
            "detail-type": "WeeklyPlanRequested",
            "source": "ipp.core",
            "account": "123456789012",
            "time": "2026-08-16T00:00:00Z",
            "region": "eu-central-1",
            "resources": [],
            "detail": {
                "event_id": "def456",
                "event_type": "WeeklyPlanRequested",
                "version": 1,
                "tenant_id": tenant_id,
                "occurred_at": "2026-08-16T00:00:00Z",
                "payload": {"plan_id": plan_id, "tag": tag, "focus_sentence": "ship things"},
            },
        }
    )


def _sqs_event(*bodies: str) -> dict:
    return {"Records": [{"messageId": f"m-{i}", "body": b} for i, b in enumerate(bodies)]}


def _handler(
    *,
    dlq: SpyDlq | None = None,
    reader: FakeReader | None = None,
    embedder: FakeEmbedder | None = None,
    writer: SpyWriter | None = None,
    embedding_reader: FakeEmbeddingReader | None = None,
    labeler: FakeLabeler | None = None,
    relationship_writer: SpyRelationshipWriter | None = None,
    relationship_reader: FakeRelationshipReader | None = None,
) -> tuple[
    Handler,
    SpyDlq,
    FakeReader,
    FakeEmbedder,
    SpyWriter,
    FakeEmbeddingReader,
    FakeLabeler,
    SpyRelationshipWriter,
    FakeRelationshipReader,
]:
    dlq = dlq or SpyDlq()
    reader = reader or FakeReader()
    embedder = embedder or FakeEmbedder()
    writer = writer or SpyWriter()
    embedding_reader = embedding_reader or FakeEmbeddingReader()
    labeler = labeler or FakeLabeler()
    relationship_writer = relationship_writer or SpyRelationshipWriter()
    relationship_reader = relationship_reader or FakeRelationshipReader()
    handler = Handler(
        dlq,
        reader,
        embedder,
        writer,
        embedding_reader,
        labeler,
        relationship_writer,
        relationship_reader,
    )
    return (
        handler,
        dlq,
        reader,
        embedder,
        writer,
        embedding_reader,
        labeler,
        relationship_writer,
        relationship_reader,
    )


def test_handle_valid_record_logs_embeds_and_skips_dlq(caplog: pytest.LogCaptureFixture) -> None:
    reader = FakeReader({("t1", "i1"): _insight()})
    embedder = FakeEmbedder(vector=(0.1, 0.2, 0.3))
    writer = SpyWriter()
    handler, dlq, *_ = _handler(reader=reader, embedder=embedder, writer=writer)

    with caplog.at_level("INFO"):
        handler.handle(_sqs_event(_envelope()))

    assert dlq.sent == []
    record = next(r for r in caplog.records if r.getMessage() == "received domain event")
    assert record.tenant_id == "t1"
    assert record.insight_id == "i1"
    assert record.event_type == "InsightEnriched"

    assert len(writer.puts) == 1
    stored = writer.puts[0]
    assert stored.insight_id == "i1"
    assert stored.tenant_id == "t1"
    assert stored.model == "text-embedding-3-small"
    assert stored.dimension == 3
    assert stored.vector == (0.1, 0.2, 0.3)


def test_handle_discovers_and_persists_relationships_above_threshold() -> None:
    """The REL 2 -> REL 3 -> REL 4 pipeline this handler is responsible for
    wiring after embed_insight: an existing insight with a near-identical
    embedding is a REL 2 candidate, the labeler's judgement clears REL 3's
    confidence threshold, and the result is posted through relationship_writer.
    """
    reader = FakeReader(
        {("t1", "i1"): _insight("i1"), ("t1", "i2"): _insight("i2", tenant_id="t1")}
    )
    embedder = FakeEmbedder(vector=(1.0, 0.0, 0.0))
    embedding_reader = FakeEmbeddingReader(
        [
            Embedding(
                insight_id="i2",
                tenant_id="t1",
                model="text-embedding-3-small",
                dimension=3,
                vector=(1.0, 0.0, 0.0),
            )
        ]
    )
    judgement = RelationJudgement(
        relation_type=RelationType.SUPPORTS, confidence=0.9, rationale="same idea"
    )
    labeler = FakeLabeler(judgement=judgement)
    relationship_writer = SpyRelationshipWriter()

    handler, *_ = _handler(
        reader=reader,
        embedder=embedder,
        embedding_reader=embedding_reader,
        labeler=labeler,
        relationship_writer=relationship_writer,
    )

    handler.handle(_sqs_event(_envelope()))

    assert len(relationship_writer.puts) == 1
    relationship = relationship_writer.puts[0]
    assert relationship.tenant_id == "t1"
    assert relationship.from_insight_id == "i1"
    assert relationship.to_insight_id == "i2"
    assert relationship.relation_type == RelationType.SUPPORTS
    assert relationship.confidence == 0.9


def test_handle_with_no_candidates_never_calls_the_labeler_or_writer() -> None:
    reader = FakeReader({("t1", "i1"): _insight()})
    relationship_writer = SpyRelationshipWriter()

    handler, *_ = _handler(reader=reader, relationship_writer=relationship_writer)

    handler.handle(_sqs_event(_envelope()))  # FakeLabeler() would raise if .label() were called

    assert relationship_writer.puts == []


def test_handle_malformed_json_routes_to_dlq_without_raising() -> None:
    handler, dlq, *_ = _handler()

    handler.handle(_sqs_event("{not json"))

    assert len(dlq.sent) == 1
    assert dlq.sent[0][0] == "{not json"


def test_handle_missing_envelope_field_routes_to_dlq() -> None:
    handler, dlq, *_ = _handler()
    body = json.dumps(
        {"detail-type": "InsightEnriched", "detail": {"event_type": "InsightEnriched"}}
    )

    handler.handle(_sqs_event(body))

    assert len(dlq.sent) == 1


def test_handle_unknown_insight_routes_to_dlq() -> None:
    handler, dlq, *_ = _handler(reader=FakeReader())  # empty — no insight i1

    handler.handle(_sqs_event(_envelope()))

    assert len(dlq.sent) == 1
    assert "i1" in dlq.sent[0][1]


def test_handle_poison_record_does_not_block_the_rest_of_the_batch(
    caplog: pytest.LogCaptureFixture,
) -> None:
    reader = FakeReader({("t1", "i2"): _insight("i2")})
    writer = SpyWriter()
    handler, dlq, *_ = _handler(reader=reader, writer=writer)

    with caplog.at_level("INFO"):
        handler.handle(_sqs_event("{not json", _envelope(insight_id="i2")))

    assert len(dlq.sent) == 1
    logged = [r for r in caplog.records if r.getMessage() == "received domain event"]
    assert len(logged) == 1
    assert logged[0].insight_id == "i2"
    assert len(writer.puts) == 1


def test_handle_dlq_send_failure_is_swallowed_as_degraded_path() -> None:
    dlq = SpyDlq(send_error=RuntimeError("sqs unavailable"))
    handler, *_ = _handler(dlq=dlq)

    handler.handle(_sqs_event("{not json"))  # must not raise

    assert len(dlq.sent) == 1


def test_handle_unexpected_error_propagates_for_redelivery(monkeypatch: pytest.MonkeyPatch) -> None:
    handler, dlq, *_ = _handler()
    monkeypatch.setattr(
        "ipp_ai.adapters.inbound.event_subscription._parse_envelope",
        lambda body: (_ for _ in ()).throw(RuntimeError("bug")),
    )

    with pytest.raises(RuntimeError):
        handler.handle(_sqs_event(_envelope()))

    assert dlq.sent == []


def test_handle_embedder_failure_propagates_for_redelivery() -> None:
    reader = FakeReader({("t1", "i1"): _insight()})
    embedder = FakeEmbedder(error=RuntimeError("embeddings provider unavailable"))
    handler, dlq, *_ = _handler(reader=reader, embedder=embedder)

    with pytest.raises(RuntimeError):
        handler.handle(_sqs_event(_envelope()))

    assert dlq.sent == []  # transient — left to propagate, not DLQ'd here


def test_handle_weekly_plan_requested_gathers_context_not_the_embed_pipeline(
    caplog: pytest.LogCaptureFixture,
) -> None:
    """PLAN 2 (IPP-104): a WeeklyPlanRequested record must route to
    gather_context, never touch the embedder/labeler/relationship_writer —
    those belong to the InsightEnriched pipeline only.
    """
    insights = [_insight(f"i{n}") for n in range(3)]
    reader = FakeReader(insights={}, by_tag={("t1", "golang"): insights})
    embedder = FakeEmbedder(error=AssertionError("embedder must not be called"))
    labeler = FakeLabeler(judgement=AssertionError("labeler must not be called"))
    relationship_writer = SpyRelationshipWriter()
    handler, dlq, *_ = _handler(
        reader=reader,
        embedder=embedder,
        labeler=labeler,
        relationship_writer=relationship_writer,
    )

    with caplog.at_level("INFO"):
        handler.handle(_sqs_event(_weekly_plan_requested_envelope(tag="golang")))

    assert dlq.sent == []
    assert relationship_writer.puts == []
    record = next(r for r in caplog.records if r.getMessage() == "selected weekly plan context")
    assert record.tenant_id == "t1"
    assert record.tag == "golang"
    assert sorted(record.insight_ids) == ["i0", "i1", "i2"]


def test_handle_weekly_plan_requested_too_few_insights_logs_insufficient_material(
    caplog: pytest.LogCaptureFixture,
) -> None:
    reader = FakeReader(insights={}, by_tag={("t1", "golang"): [_insight("i0")]})
    handler, dlq, *_ = _handler(reader=reader)

    with caplog.at_level("INFO"):
        handler.handle(_sqs_event(_weekly_plan_requested_envelope(tag="golang")))

    assert dlq.sent == []
    record = next(
        r for r in caplog.records if r.getMessage() == "not enough material to plan a week"
    )
    assert record.tenant_id == "t1"
    assert record.tag == "golang"
    assert record.insight_count == 1
