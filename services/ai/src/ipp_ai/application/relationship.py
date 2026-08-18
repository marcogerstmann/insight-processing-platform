"""Labels candidate pairs with an LLM judgement — IPP-99 / REL 3.

Sits after REL 2's select_candidates (domain/candidate.py): that function
narrows a tenant's whole corpus down to a shortlist by cheap arithmetic;
this spends the LLM call REL 2 exists to avoid making O(n) times, on that
shortlist only.

Failure here is soft, unlike embed_insight's (application/embedding.py):
there is no already-durable write a relationship judgement protects, and
"no relationships found" is a legitimate outcome for a pair the model
can't characterize, a timeout, or a malformed response (RelationType
rejects an unrecognized string outright rather than coercing it — see
domain/relationship.py). One bad pair must never cost the rest of the
run, so each call is isolated: logged and skipped, never raised.
"""

from __future__ import annotations

import logging

from ipp_ai.domain.insight import Insight
from ipp_ai.domain.relationship import Relationship
from ipp_ai.ports import RelationLabeler

logger = logging.getLogger(__name__)

# Picked, not measured — same caveat as REL 2's CANDIDATE_* constants:
# there is no accept/reject data yet to tune against.
#
# Separate from REL 2's CANDIDATE_TOP_K on purpose: candidate selection is
# free arithmetic and can afford to shortlist generously, LLM calls can't
# — this is what actually bounds cost per new insight.
MAX_PAIRS_PER_RUN = 5

# Below this, a judgement is discarded rather than stored as a weak edge:
# an edge is presented to the user as a claim, not a hint, so a
# low-confidence "maybe related" is worse than no edge at all.
RELATIONSHIP_CONFIDENCE_THRESHOLD = 0.6


def label_relationships(
    tenant_id: str,
    query: Insight,
    candidates: list[Insight],
    *,
    labeler: RelationLabeler,
) -> list[Relationship]:
    """Labels up to MAX_PAIRS_PER_RUN candidates against `query`.

    Never raises: a failing or below-threshold pair is logged and
    skipped, not surfaced. An empty result is a normal outcome.
    """
    relationships: list[Relationship] = []
    for candidate in candidates[:MAX_PAIRS_PER_RUN]:
        try:
            judgement = labeler.label(query.text, candidate.text)
        except Exception:
            logger.warning(
                "relation labeling failed, skipping pair",
                exc_info=True,
                extra={
                    "tenant_id": tenant_id,
                    "insight_id": query.id,
                    "candidate_insight_id": candidate.id,
                },
            )
            continue

        if judgement.confidence < RELATIONSHIP_CONFIDENCE_THRESHOLD:
            continue

        relationships.append(
            Relationship(
                tenant_id=tenant_id,
                from_insight_id=query.id,
                to_insight_id=candidate.id,
                relation_type=judgement.relation_type,
                confidence=judgement.confidence,
                rationale=judgement.rationale,
            )
        )
    return relationships
