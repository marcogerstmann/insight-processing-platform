from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime

from ipp_ai.application.context_gathering import gather_context
from ipp_ai.domain.insight import Insight
from ipp_ai.domain.plan_context import InsufficientMaterial, SelectedContext
from ipp_ai.domain.relationship import RelatedInsight, RelationType

_NOW = datetime.fromisoformat("2026-08-22T00:00:00")


@dataclass
class FakeInsightReader:
    by_tag: dict[tuple[str, str], list[Insight]] = field(default_factory=dict)

    def list_by_tag(self, tenant_id: str, tag: str) -> list[Insight]:
        return self.by_tag.get((tenant_id, tag), [])


@dataclass
class FakeRelationshipReader:
    edges: dict[tuple[str, str], list[RelatedInsight]] = field(default_factory=dict)

    def list_by_insight(self, tenant_id: str, insight_id: str) -> list[RelatedInsight]:
        return self.edges.get((tenant_id, insight_id), [])


def _insight(insight_id: str) -> Insight:
    return Insight(
        id=insight_id,
        tenant_id="t1",
        source="readwise",
        text="hello world",
        notes="",
        highlighted_at=_NOW,
    )


def test_gather_context_loads_the_tag_and_every_insights_relationships() -> None:
    insights = [_insight("i1"), _insight("i2"), _insight("i3")]
    edge = RelatedInsight(
        insight_id="i2",
        text="hello world",
        relation_type=RelationType.SUPPORTS,
        confidence=0.9,
        rationale="because reasons",
    )
    reader = FakeInsightReader(by_tag={("t1", "golang"): insights})
    relationship_reader = FakeRelationshipReader(edges={("t1", "i1"): [edge]})

    result = gather_context(
        "t1", "golang", insight_reader=reader, relationship_reader=relationship_reader, now=_NOW
    )

    assert isinstance(result, SelectedContext)
    assert {i.id for i in result.insights} == {"i1", "i2", "i3"}
    assert len(result.edges) == 1


def test_gather_context_empty_tag_returns_insufficient_material() -> None:
    reader = FakeInsightReader()
    relationship_reader = FakeRelationshipReader()

    result = gather_context(
        "t1", "golang", insight_reader=reader, relationship_reader=relationship_reader, now=_NOW
    )

    assert result == InsufficientMaterial(tag="golang", insight_count=0)
