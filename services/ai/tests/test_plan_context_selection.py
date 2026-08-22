from __future__ import annotations

from datetime import datetime, timedelta

from ipp_ai.domain.insight import Insight
from ipp_ai.domain.plan_context import (
    CONTEXT_TOKEN_BUDGET,
    MAX_SELECTED_INSIGHTS,
    MIN_INSIGHTS_FOR_PLAN,
    InsufficientMaterial,
    SelectedContext,
    select_context,
)
from ipp_ai.domain.relationship import RelatedInsight, RelationType

_NOW = datetime.fromisoformat("2026-08-22T00:00:00")


def _insight(
    insight_id: str, *, age_days: float = 0, text: str = "hello", notes: str = ""
) -> Insight:
    return Insight(
        id=insight_id,
        tenant_id="t1",
        source="readwise",
        text=text,
        notes=notes,
        highlighted_at=_NOW - timedelta(days=age_days),
    )


def _edge(other_id: str, *, confidence: float = 0.9) -> RelatedInsight:
    return RelatedInsight(
        insight_id=other_id,
        text="other text",
        relation_type=RelationType.SUPPORTS,
        confidence=confidence,
        rationale="because reasons",
    )


def test_below_minimum_corpus_returns_insufficient_material() -> None:
    insights = [_insight(f"i{n}") for n in range(MIN_INSIGHTS_FOR_PLAN - 1)]

    result = select_context("golang", insights, {}, now=_NOW)

    assert result == InsufficientMaterial(tag="golang", insight_count=len(insights))


def test_empty_corpus_returns_insufficient_material() -> None:
    result = select_context("golang", [], {}, now=_NOW)

    assert result == InsufficientMaterial(tag="golang", insight_count=0)


def test_happy_path_selects_all_insights_within_cap_and_budget() -> None:
    insights = [_insight(f"i{n}", age_days=n) for n in range(MIN_INSIGHTS_FOR_PLAN)]

    result = select_context("golang", insights, {}, now=_NOW)

    assert isinstance(result, SelectedContext)
    assert result.tag == "golang"
    assert {i.id for i in result.insights} == {i.id for i in insights}
    assert result.estimated_tokens > 0


def test_more_recent_insight_is_ranked_and_selected_first_when_capped() -> None:
    # More candidates than the cap, all otherwise identical: only recency
    # tells them apart, so the freshest MAX_SELECTED_INSIGHTS must win.
    insights = [_insight(f"i{n}", age_days=n) for n in range(MAX_SELECTED_INSIGHTS + 5)]

    result = select_context("golang", insights, {}, now=_NOW)

    assert isinstance(result, SelectedContext)
    assert len(result.insights) == MAX_SELECTED_INSIGHTS
    selected_ids = {i.id for i in result.insights}
    assert selected_ids == {f"i{n}" for n in range(MAX_SELECTED_INSIGHTS)}


def test_centrality_ranks_a_well_connected_insight_above_an_equally_recent_isolated_one() -> None:
    # Same age, so recency contributes identically to both — only
    # centrality can be what separates them in rank order.
    connected = _insight("i-connected", age_days=5)
    isolated = _insight("i-isolated", age_days=5)
    linked = [_insight(f"i-linked{n}", age_days=5) for n in range(3)]
    insights = [connected, isolated, *linked]

    relationships = {
        "i-connected": [_edge(i.id) for i in linked],
        **{i.id: [_edge("i-connected")] for i in linked},
    }

    result = select_context("golang", insights, relationships, now=_NOW)

    assert isinstance(result, SelectedContext)
    ranked_ids = [i.id for i in result.insights]
    assert ranked_ids.index("i-connected") < ranked_ids.index("i-isolated")


def test_token_budget_drops_whole_insights_never_truncates_text() -> None:
    huge = _insight("i-huge", age_days=0, text="x" * (CONTEXT_TOKEN_BUDGET * 4 * 2))
    small = [_insight(f"i-small{n}", age_days=n + 1, text="short") for n in range(3)]
    insights = [huge, *small]

    result = select_context("golang", insights, {}, now=_NOW)

    assert isinstance(result, SelectedContext)
    # The huge insight is ranked first (most recent) but doesn't fit the
    # budget; it must be skipped whole, not clipped down to fit.
    assert "i-huge" not in {i.id for i in result.insights}
    for insight in result.insights:
        assert insight.text in ("short",)
    assert result.estimated_tokens <= CONTEXT_TOKEN_BUDGET


def test_selected_insights_dropping_below_minimum_after_budget_is_insufficient_material() -> None:
    huge = _insight("i-huge", age_days=0, text="x" * (CONTEXT_TOKEN_BUDGET * 4 * 2))
    tiny_count = MIN_INSIGHTS_FOR_PLAN - 1
    tiny = [_insight(f"i-tiny{n}", age_days=n + 1, text="short") for n in range(tiny_count)]
    insights = [huge, *tiny]

    result = select_context("golang", insights, {}, now=_NOW)

    assert result == InsufficientMaterial(tag="golang", insight_count=len(insights))


def test_edges_are_included_only_between_two_selected_insights_and_not_duplicated() -> None:
    a, b, c = _insight("a", age_days=0), _insight("b", age_days=1), _insight("c", age_days=2)
    # "outsider" (referenced only by id below) is never added to `insights`,
    # standing in for a related insight that isn't a member of this tag —
    # its edge must never appear in the output.
    filler = [_insight(f"filler{n}", age_days=10 + n) for n in range(MAX_SELECTED_INSIGHTS)]
    insights = [a, b, c, *filler]

    relationships = {
        "a": [_edge("b"), _edge("outsider")],
        "b": [_edge("a")],
        "outsider": [_edge("a")],
    }

    result = select_context("golang", insights, relationships, now=_NOW)

    assert isinstance(result, SelectedContext)
    selected_ids = {i.id for i in result.insights}
    assert "outsider" not in selected_ids
    assert len(result.edges) == 1
    edge = result.edges[0]
    assert {edge.from_insight_id, edge.to_insight_id} == {"a", "b"}


def test_deterministic_for_the_same_inputs() -> None:
    insights = [_insight(f"i{n}", age_days=n) for n in range(MIN_INSIGHTS_FOR_PLAN + 2)]

    first = select_context("golang", insights, {}, now=_NOW)
    second = select_context("golang", insights, {}, now=_NOW)

    assert first == second
