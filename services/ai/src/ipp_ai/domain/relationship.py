"""Relationship judgement — pure value types, no I/O.

The read side's counterpart to internal/domain.Relationship (IPP-100),
minus the fields only persistence needs (`DiscoveredAt`, the deterministic
`sk`): this is what REL 3 produces in memory, one candidate pair at a time.
REL 4 is what turns a Relationship into a stored, bidirectional edge.
"""

from __future__ import annotations

from dataclasses import dataclass
from enum import StrEnum


class RelationType(StrEnum):
    """IPP-99's fixed enum. A value outside this set is not a RelationType —
    construct via `RelationType(raw)` and let the ValueError propagate;
    there is no `.get()`-style coercion path from an unrecognized string.
    """

    SUPPORTS = "supports"
    CONTRADICTS = "contradicts"
    EXTENDS = "extends"
    EXAMPLE_OF = "example_of"
    SAME_TOPIC = "same_topic"


@dataclass(frozen=True)
class RelationJudgement:
    """One LLM call's verdict on a single candidate pair, before the
    confidence threshold (application/relationship.py) decides whether it
    becomes a Relationship.
    """

    relation_type: RelationType
    confidence: float
    rationale: str


@dataclass(frozen=True)
class Relationship:
    """A judgement that cleared the confidence threshold, addressed to a
    specific ordered pair of insights.
    """

    tenant_id: str
    from_insight_id: str
    to_insight_id: str
    relation_type: RelationType
    confidence: float
    rationale: str


@dataclass(frozen=True)
class RelatedInsight:
    """One edge from an insight's perspective — the read side's counterpart
    to internal/domain.RelatedInsight (REL 6/IPP-102). `insight_id` is
    whichever insight is on the *other* end from the one you queried; `text`
    is denormalized onto the edge at write time (see the Go adapter), so
    reading it never needs a second fetch.
    """

    insight_id: str
    text: str
    relation_type: RelationType
    confidence: float
    rationale: str
