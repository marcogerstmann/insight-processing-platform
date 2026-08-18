from __future__ import annotations

import pytest

from ipp_ai.domain.candidate import (
    CANDIDATE_SIMILARITY_THRESHOLD,
    CANDIDATE_TOP_K,
    select_candidates,
)
from ipp_ai.domain.embedding import Embedding, IncomparableEmbeddings


def _embedding(
    insight_id: str,
    vector: tuple[float, ...],
    *,
    model: str = "text-embedding-3-small",
    dimension: int = 512,
) -> Embedding:
    return Embedding(
        insight_id=insight_id,
        tenant_id="t1",
        model=model,
        dimension=dimension,
        vector=vector,
    )


def test_an_obvious_match_is_returned_with_a_high_score() -> None:
    query = _embedding("q", (1.0, 0.0))
    match = _embedding("match", (1.0, 0.0))

    result = select_candidates(query, [match])

    assert result == [("match", pytest.approx(1.0))]


def test_an_obvious_non_match_is_excluded() -> None:
    query = _embedding("q", (1.0, 0.0))
    unrelated = _embedding("unrelated", (0.0, 1.0))

    assert select_candidates(query, [unrelated]) == []


def test_threshold_boundary_is_inclusive() -> None:
    query = _embedding("q", (1.0, 0.0))
    # cos(angle) == CANDIDATE_SIMILARITY_THRESHOLD exactly.
    import math

    angle = math.acos(CANDIDATE_SIMILARITY_THRESHOLD)
    on_boundary = _embedding("on-boundary", (math.cos(angle), math.sin(angle)))

    result = select_candidates(query, [on_boundary])

    assert [insight_id for insight_id, _ in result] == ["on-boundary"]


def test_just_below_threshold_is_excluded() -> None:
    query = _embedding("q", (1.0, 0.0))
    import math

    angle = math.acos(CANDIDATE_SIMILARITY_THRESHOLD) + 0.01
    below = _embedding("below", (math.cos(angle), math.sin(angle)))

    assert select_candidates(query, [below]) == []


def test_empty_corpus_returns_no_candidates() -> None:
    query = _embedding("q", (1.0, 0.0))

    assert select_candidates(query, []) == []


def test_self_match_is_excluded() -> None:
    query = _embedding("q", (1.0, 0.0))
    self_copy = _embedding("q", (1.0, 0.0))

    assert select_candidates(query, [self_copy]) == []


def test_already_linked_insights_are_excluded() -> None:
    query = _embedding("q", (1.0, 0.0))
    linked = _embedding("already-linked", (1.0, 0.0))

    result = select_candidates(query, [linked], already_linked=frozenset({"already-linked"}))

    assert result == []


def test_results_are_capped_at_top_k_and_sorted_descending() -> None:
    query = _embedding("q", (1.0, 0.0))
    candidates = [_embedding(f"c{i}", (1.0, 0.01 * i)) for i in range(CANDIDATE_TOP_K + 5)]

    result = select_candidates(query, candidates)

    assert len(result) == CANDIDATE_TOP_K
    scores = [score for _, score in result]
    assert scores == sorted(scores, reverse=True)


def test_comparing_across_vector_spaces_raises_rather_than_scoring() -> None:
    query = _embedding("q", (1.0, 0.0))
    incompatible = _embedding("other-model", (0.1, 0.2), model="voyage-3", dimension=1024)

    with pytest.raises(IncomparableEmbeddings):
        select_candidates(query, [incompatible])
