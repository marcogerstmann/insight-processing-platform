from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime

from ipp_ai.application.relationship import (
    MAX_PAIRS_PER_RUN,
    RELATIONSHIP_CONFIDENCE_THRESHOLD,
    label_relationships,
)
from ipp_ai.domain.insight import Insight
from ipp_ai.domain.relationship import RelationJudgement, RelationType


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
