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

from ipp_ai.domain.candidate import select_candidates
from ipp_ai.domain.embedding import Embedding
from ipp_ai.domain.insight import Insight
from ipp_ai.domain.relationship import Relationship
from ipp_ai.ports import EmbeddingReader, InsightReader, RelationLabeler, RelationshipWriter

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


def discover_relationships(
    tenant_id: str,
    query_embedding: Embedding,
    *,
    insight_reader: InsightReader,
    embedding_reader: EmbeddingReader,
    labeler: RelationLabeler,
    relationship_writer: RelationshipWriter,
) -> None:
    """The REL 2 -> REL 3 -> REL 4 pipeline for one newly embedded insight.

    Runs straight after embed_insight in the Lambda handler. Not itself
    soft-fail: label_relationships already swallows per-pair LLM failures
    (a candidate that can't be judged is not a bug), but a
    relationship_writer.put failure propagates, same as embed_insight's own
    failures, so the runtime redelivers the event. Safe to redeliver — both
    the embedding upsert and the Go endpoint's write are idempotent by key.
    """
    query_insight = insight_reader.get_by_id(tenant_id, query_embedding.insight_id)
    if query_insight is None:
        return  # insight deleted since it was embedded; nothing to relate

    candidate_ids = select_candidates(query_embedding, embedding_reader.list_by_tenant(tenant_id))
    candidates = [
        insight
        for insight_id, _score in candidate_ids
        if (insight := insight_reader.get_by_id(tenant_id, insight_id)) is not None
    ]

    for relationship in label_relationships(tenant_id, query_insight, candidates, labeler=labeler):
        relationship_writer.put(relationship)
