"""Candidate selection for the Relationship Agent — IPP-98.

Pure, no I/O: loading the query's and the tenant's stored vectors is the
adapter's job, same boundary as everywhere else in this service (see
services/ai/README.md). This is REL 2's brute-force alternative to a
vector database — domain/embedding.py's cosine_similarity already argues
why that trade-off is deliberate; this module is the batched form of it.

Ceiling: a linear scan over every one of the tenant's stored vectors,
O(n) per query. At a few thousand insights that is low-single-digit
milliseconds even before numpy; the vectorized dot product below buys
headroom well past that. Revisit with an ANN index (e.g. pgvector,
OpenSearch k-NN) once a tenant's corpus crosses roughly 100k insights —
below that, an index is infrastructure and Terraform surface bought for
no measurable benefit.
"""

from __future__ import annotations

import numpy as np

from ipp_ai.domain.embedding import Embedding, IncomparableEmbeddings

# Picked, not measured — there is no relationship-judgment data yet to
# tune against. TOP_K bounds how many LLM calls (REL 3) one new insight
# can trigger; THRESHOLD is set low on purpose, to admit false positives
# for the LLM to filter rather than silently drop a true match. Revisit
# both once REL 3's accept/reject verdicts give a real signal.
CANDIDATE_TOP_K = 10
CANDIDATE_SIMILARITY_THRESHOLD = 0.75


def select_candidates(
    query: Embedding,
    candidates: list[Embedding],
    *,
    already_linked: frozenset[str] = frozenset(),
) -> list[tuple[str, float]]:
    """Top-K candidates for `query` by cosine similarity, above threshold.

    Excludes `query` itself and any insight_id in `already_linked`.
    Returns (insight_id, score) pairs, highest score first.
    """
    pool = [
        c
        for c in candidates
        if c.insight_id != query.insight_id and c.insight_id not in already_linked
    ]
    if not pool:
        return []

    for c in pool:
        if c.model != query.model or c.dimension != query.dimension:
            raise IncomparableEmbeddings(
                f"cannot compare {query.model}/{query.dimension} with "
                f"{c.model}/{c.dimension} — different vector spaces; "
                "re-embed one side before comparing"
            )

    query_vec = np.array(query.vector)
    query_norm = np.linalg.norm(query_vec)
    if query_norm == 0:
        return []

    matrix = np.array([c.vector for c in pool])
    norms = np.linalg.norm(matrix, axis=1)
    dots = matrix @ query_vec
    scores = np.divide(dots, norms * query_norm, out=np.zeros_like(dots), where=norms != 0)

    ranked = sorted(
        zip((c.insight_id for c in pool), scores.tolist(), strict=True),
        key=lambda pair: pair[1],
        reverse=True,
    )
    above_threshold = [pair for pair in ranked if pair[1] >= CANDIDATE_SIMILARITY_THRESHOLD]
    return above_threshold[:CANDIDATE_TOP_K]
