from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime

from ipp_ai.application.action_generation import generate_actions
from ipp_ai.domain.action import Action
from ipp_ai.domain.insight import Insight
from ipp_ai.domain.plan_context import ContextEdge, SelectedContext


@dataclass
class StubGenerator:
    drafts: list[Action]
    received: list[tuple[str, tuple[str, ...], int]] = field(default_factory=list)

    def generate(
        self, focus_sentence: str, insights: list[Insight], edges: tuple[ContextEdge, ...]
    ) -> list[Action]:
        self.received.append((focus_sentence, tuple(i.id for i in insights), len(edges)))
        return self.drafts


def _insight(insight_id: str) -> Insight:
    return Insight(
        id=insight_id,
        tenant_id="t1",
        source="readwise",
        text="hello world",
        notes="",
        highlighted_at=datetime.fromisoformat("2026-01-01T00:00:00"),
    )


def _context(*insight_ids: str) -> SelectedContext:
    insights = tuple(_insight(i) for i in insight_ids)
    return SelectedContext(tag="golang", insights=insights, edges=(), estimated_tokens=10)


def test_a_valid_response_survives_unchanged() -> None:
    draft = Action(
        title="Ship it", why="Directly serves the focus.", supporting_insight_ids=("i1", "i2")
    )
    generator = StubGenerator(drafts=[draft])
    context = _context("i1", "i2", "i3")

    result = generate_actions("t1", "ship the draft", context, generator=generator)

    assert result == [draft]
    assert generator.received == [("ship the draft", ("i1", "i2", "i3"), 0)]


def test_a_hallucinated_id_is_dropped_but_the_action_survives_on_its_real_citation() -> None:
    draft = Action(title="Ship it", why="why", supporting_insight_ids=("i1", "made-up-id"))
    generator = StubGenerator(drafts=[draft])
    context = _context("i1", "i2")

    result = generate_actions("t1", "focus", context, generator=generator)

    assert len(result) == 1
    assert result[0].supporting_insight_ids == ("i1",)


def test_an_action_with_only_hallucinated_ids_is_discarded() -> None:
    draft = Action(title="Ship it", why="why", supporting_insight_ids=("made-up-id",))
    generator = StubGenerator(drafts=[draft])
    context = _context("i1", "i2")

    assert generate_actions("t1", "focus", context, generator=generator) == []


def test_an_empty_response_returns_an_empty_list_never_padded() -> None:
    generator = StubGenerator(drafts=[])
    context = _context("i1", "i2", "i3")

    assert generate_actions("t1", "focus", context, generator=generator) == []


def test_fewer_than_three_valid_actions_is_returned_as_is() -> None:
    draft = Action(title="Only one", why="why", supporting_insight_ids=("i1",))
    generator = StubGenerator(drafts=[draft])
    context = _context("i1", "i2", "i3")

    result = generate_actions("t1", "focus", context, generator=generator)

    assert len(result) == 1
