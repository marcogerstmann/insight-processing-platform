"""Bounded context selection for the Action Agent — PLAN 2/IPP-104.

Pure, no I/O: loading a tag's insights and their relationships is
application/context_gathering.py's job, same boundary as domain/candidate.py's
REL 2. This is the engineering IPP-104 is actually about — deciding *what not
to send* so an LLM call's cost stays predictable no matter how large a tag's
corpus grows, rather than "send everything" silently breaking past a few
hundred insights.

Scoring is recency + relationship centrality, the same weighted-component
shape as internal/domain.TagRelevanceScore (exponential half-life decay,
saturating counts, weights summing to 1) translated from tag-level ranking to
insight-level selection.
"""

from __future__ import annotations

import math
from dataclasses import dataclass
from datetime import datetime

from ipp_ai.domain.insight import Insight
from ipp_ai.domain.relationship import RelatedInsight

# Named cap (IPP-104's AC): bounds how many insights one plan's context can
# ever include, independent of the token budget below. This is what keeps
# selection cheap even before the budget kicks in — no need to score
# thousands of insights just to find CONTEXT_TOKEN_BUDGET is already blown.
MAX_SELECTED_INSIGHTS = 12

# TRADE-OFF: character count as a stand-in for token count, the same call
# adapters/outbound/openai.py's _MAX_INPUT_CHARS makes — no tokenizer
# dependency for one budget guard. ~4 chars/token is a common English-text
# rule of thumb; swap in a real tokenizer if the budget ever visibly
# over/under-shoots.
_CHARS_PER_TOKEN = 4
CONTEXT_TOKEN_BUDGET = 4000

# Split between "was this recently highlighted" and "how connected is this
# insight to the rest of the tag's graph" — same weighted-component shape as
# internal/domain.TagRelevanceScore, sized so a well-connected insight can
# still outrank a slightly more recent one rather than recency dominating
# outright. Picked, not measured — there is no usage data yet to tune against.
_RECENCY_WEIGHT = 0.6
_CENTRALITY_WEIGHT = 0.4

# Same half-life logic as internal/domain.tagRelevanceHalfLife: a highlight
# from a week ago has already faded halfway.
_RECENCY_HALF_LIFE_HOURS = 7 * 24.0

# Saturation point for centrality, mirroring tagRelevanceDensitySaturation:
# an insight linked to 3 others already reads as "well-connected" at a
# personal tag's usual size.
_CENTRALITY_SATURATION = 3.0

# Below this many insights there isn't enough material to plan a week from —
# PLAN 3 needs a handful of distinct threads to draw 3-5 actions out of, not
# one highlight read back to itself. Picked, not measured.
MIN_INSIGHTS_FOR_PLAN = 3


@dataclass(frozen=True)
class ContextEdge:
    """One relationship between two insights that both made the selection
    cut — the capped, read-side counterpart of domain.relationship's
    RelatedInsight, addressed as an ordered pair rather than "from one
    insight's perspective" since the context sent to the LLM has no single
    perspective insight.
    """

    from_insight_id: str
    to_insight_id: str
    relation_type: str
    confidence: float
    rationale: str


@dataclass(frozen=True)
class SelectedContext:
    """The bounded slice of a tag's corpus PLAN 3's agent gets to see."""

    tag: str
    insights: tuple[Insight, ...]
    edges: tuple[ContextEdge, ...]
    estimated_tokens: int


@dataclass(frozen=True)
class InsufficientMaterial:
    """The tag has too few insights to plan a week from — a legitimate
    outcome (IPP-104's AC), not an error: the caller logs it and stops,
    rather than handing PLAN 3 a corpus too thin to avoid a hallucinated plan.
    """

    tag: str
    insight_count: int


def select_context(
    tag: str,
    insights: list[Insight],
    relationships: dict[str, list[RelatedInsight]],
    *,
    now: datetime,
) -> SelectedContext | InsufficientMaterial:
    """Selects a capped, budget-bounded slice of `insights` for `tag`.

    `relationships` maps each of `insights`' ids to its edges
    (RelationshipReader.list_by_insight's shape) — used only to score
    centrality among insights already known to carry `tag`, never to reach
    outside it.

    Two passes, deliberately: rank *every* candidate insight by score first,
    then admit ranked insights one at a time until MAX_SELECTED_INSIGHTS or
    CONTEXT_TOKEN_BUDGET is hit. Ranking first means a highly-central insight
    is never dropped just because a pile of less relevant, more numerous
    insights happened to fill the cap first. The budget truncates by
    dropping whole insights, never mid-text (IPP-104's AC) — an insight that
    doesn't fit is skipped, not clipped, and a smaller one later in rank
    order may still fit after it.
    """
    if len(insights) < MIN_INSIGHTS_FOR_PLAN:
        return InsufficientMaterial(tag=tag, insight_count=len(insights))

    degree = {insight_id: len(edges) for insight_id, edges in relationships.items()}
    ranked = sorted(insights, key=lambda i: _score(i, degree.get(i.id, 0), now), reverse=True)

    selected: list[Insight] = []
    tokens = 0
    for insight in ranked:
        if len(selected) >= MAX_SELECTED_INSIGHTS:
            break
        cost = _estimate_tokens(insight)
        if tokens + cost > CONTEXT_TOKEN_BUDGET:
            continue  # this one doesn't fit; a smaller one later in rank order might
        selected.append(insight)
        tokens += cost

    if len(selected) < MIN_INSIGHTS_FOR_PLAN:
        return InsufficientMaterial(tag=tag, insight_count=len(insights))

    selected_ids = {i.id for i in selected}
    edges = tuple(
        ContextEdge(
            from_insight_id=insight_id,
            to_insight_id=edge.insight_id,
            relation_type=str(edge.relation_type),
            confidence=edge.confidence,
            rationale=edge.rationale,
        )
        for insight_id in selected_ids
        for edge in relationships.get(insight_id, [])
        # Both directions of every edge are present in `relationships`
        # (RelationshipReader mirrors the Go adapter's bidirectional write);
        # this ordering keeps each edge in the result exactly once.
        if edge.insight_id in selected_ids and insight_id < edge.insight_id
    )

    return SelectedContext(tag=tag, insights=tuple(selected), edges=edges, estimated_tokens=tokens)


def _score(insight: Insight, degree: int, now: datetime) -> float:
    age_hours = max((now - insight.highlighted_at).total_seconds() / 3600.0, 0.0)
    recency = math.exp(-math.log(2) * age_hours / _RECENCY_HALF_LIFE_HOURS)
    centrality = degree / (degree + _CENTRALITY_SATURATION)
    return _RECENCY_WEIGHT * recency + _CENTRALITY_WEIGHT * centrality


def _estimate_tokens(insight: Insight) -> int:
    return (len(insight.text) + len(insight.notes)) // _CHARS_PER_TOKEN
