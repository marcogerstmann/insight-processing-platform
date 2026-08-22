from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime

from ipp_ai.application.relationship import (
    MAX_PAIRS_PER_RUN,
    RELATIONSHIP_CONFIDENCE_THRESHOLD,
    discover_relationships,
    label_relationships,
)
from ipp_ai.domain.embedding import Embedding
from ipp_ai.domain.insight import Insight
from ipp_ai.domain.relationship import RelationJudgement, Relationship, RelationType


@dataclass
class StubLabeler:
    judgements: list[RelationJudgement | Exception] = field(default_factory=list)
    received: list[tuple[str, str]] = field(default_factory=list)

    def label(self, from_text: str, to_text: str) -> RelationJudgement:
        self.received.append((from_text, to_text))
        result = self.judgements[len(self.received) - 1]
        if isinstance(result, Exception):
            raise result
        return result


def _insight(insight_id: str, text: str = "highlight text") -> Insight:
    return Insight(
        id=insight_id,
        tenant_id="t1",
        source="readwise",
        text=text,
        notes="",
        highlighted_at=datetime.fromisoformat("2026-01-01T00:00:00"),
    )


def _judgement(
    *, relation_type: RelationType = RelationType.SUPPORTS, confidence: float = 0.9
) -> RelationJudgement:
    return RelationJudgement(relation_type=relation_type, confidence=confidence, rationale="why")


def test_a_high_confidence_pair_becomes_a_relationship() -> None:
    query = _insight("q", "query text")
    candidate = _insight("c1", "candidate text")
    labeler = StubLabeler(judgements=[_judgement(confidence=0.9)])

    result = label_relationships("t1", query, [candidate], labeler=labeler)

    assert len(result) == 1
    relationship = result[0]
    assert relationship.tenant_id == "t1"
    assert relationship.from_insight_id == "q"
    assert relationship.to_insight_id == "c1"
    assert relationship.relation_type is RelationType.SUPPORTS
    assert relationship.confidence == 0.9
    assert labeler.received == [("query text", "candidate text")]


def test_a_pair_below_the_confidence_threshold_is_discarded() -> None:
    query = _insight("q")
    candidate = _insight("c1")
    below = RELATIONSHIP_CONFIDENCE_THRESHOLD - 0.01
    labeler = StubLabeler(judgements=[_judgement(confidence=below)])

    assert label_relationships("t1", query, [candidate], labeler=labeler) == []


def test_a_pair_exactly_at_the_confidence_threshold_is_kept() -> None:
    query = _insight("q")
    candidate = _insight("c1")
    labeler = StubLabeler(judgements=[_judgement(confidence=RELATIONSHIP_CONFIDENCE_THRESHOLD)])

    assert len(label_relationships("t1", query, [candidate], labeler=labeler)) == 1


def test_a_failing_pair_is_skipped_without_stopping_the_run() -> None:
    query = _insight("q")
    candidates = [_insight("c1"), _insight("c2")]
    labeler = StubLabeler(judgements=[RuntimeError("timeout"), _judgement(confidence=0.9)])

    result = label_relationships("t1", query, candidates, labeler=labeler)

    assert len(result) == 1
    assert result[0].to_insight_id == "c2"


def test_no_candidates_clear_the_run_returns_an_empty_list() -> None:
    query = _insight("q")
    candidates = [_insight("c1"), _insight("c2")]
    labeler = StubLabeler(judgements=[RuntimeError("x"), RuntimeError("y")])

    assert label_relationships("t1", query, candidates, labeler=labeler) == []


def test_candidates_beyond_max_pairs_per_run_are_never_sent_to_the_labeler() -> None:
    query = _insight("q")
    candidates = [_insight(f"c{i}") for i in range(MAX_PAIRS_PER_RUN + 3)]
    labeler = StubLabeler(judgements=[_judgement(confidence=0.9)] * MAX_PAIRS_PER_RUN)

    result = label_relationships("t1", query, candidates, labeler=labeler)

    assert len(labeler.received) == MAX_PAIRS_PER_RUN
    assert len(result) == MAX_PAIRS_PER_RUN


@dataclass
class FakeInsightReader:
    insights: dict[str, Insight] = field(default_factory=dict)

    def get_by_id(self, tenant_id: str, insight_id: str) -> Insight | None:
        return self.insights.get(insight_id)


@dataclass
class FakeEmbeddingReader:
    embeddings: list[Embedding] = field(default_factory=list)

    def list_by_tenant(self, tenant_id: str) -> list[Embedding]:
        return self.embeddings


@dataclass
class SpyRelationshipWriter:
    puts: list[Relationship] = field(default_factory=list)

    def put(self, relationship: Relationship) -> None:
        self.puts.append(relationship)


def _embedding(insight_id: str, vector: tuple[float, ...]) -> Embedding:
    return Embedding(
        insight_id=insight_id, tenant_id="t1", model="m", dimension=len(vector), vector=vector
    )


def test_discover_relationships_persists_labeled_candidates_above_threshold() -> None:
    query_insight = _insight("q", "query text")
    candidate_insight = _insight("c1", "candidate text")
    insight_reader = FakeInsightReader({"q": query_insight, "c1": candidate_insight})
    embedding_reader = FakeEmbeddingReader([_embedding("c1", (1.0, 0.0))])
    labeler = StubLabeler(judgements=[_judgement(confidence=0.9)])
    relationship_writer = SpyRelationshipWriter()

    discover_relationships(
        "t1",
        _embedding("q", (1.0, 0.0)),
        insight_reader=insight_reader,
        embedding_reader=embedding_reader,
        labeler=labeler,
        relationship_writer=relationship_writer,
    )

    assert len(relationship_writer.puts) == 1
    assert relationship_writer.puts[0].to_insight_id == "c1"


def test_discover_relationships_with_no_candidates_never_calls_the_labeler() -> None:
    query_insight = _insight("q", "query text")
    insight_reader = FakeInsightReader({"q": query_insight})
    embedding_reader = FakeEmbeddingReader([])  # only the query's own embedding exists
    labeler = StubLabeler(judgements=[])
    relationship_writer = SpyRelationshipWriter()

    discover_relationships(
        "t1",
        _embedding("q", (1.0, 0.0)),
        insight_reader=insight_reader,
        embedding_reader=embedding_reader,
        labeler=labeler,
        relationship_writer=relationship_writer,
    )

    assert labeler.received == []
    assert relationship_writer.puts == []


def test_discover_relationships_returns_quietly_if_the_query_insight_is_gone() -> None:
    """The insight could be deleted between InsightEnriched firing and this
    running — not a bug, just nothing left to relate.
    """
    insight_reader = FakeInsightReader({})  # empty
    embedding_reader = FakeEmbeddingReader([_embedding("c1", (1.0, 0.0))])
    labeler = StubLabeler(judgements=[])
    relationship_writer = SpyRelationshipWriter()

    discover_relationships(
        "t1",
        _embedding("q", (1.0, 0.0)),
        insight_reader=insight_reader,
        embedding_reader=embedding_reader,
        labeler=labeler,
        relationship_writer=relationship_writer,
    )

    assert relationship_writer.puts == []
