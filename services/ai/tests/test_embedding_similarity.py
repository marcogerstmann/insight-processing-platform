from __future__ import annotations

import pytest

from ipp_ai.domain.embedding import Embedding, IncomparableEmbeddings, cosine_similarity


def _embedding(
    vector: tuple[float, ...],
    *,
    model: str = "text-embedding-3-small",
    dimension: int = 512,
) -> Embedding:
    return Embedding(
        insight_id="i1",
        tenant_id="t1",
        model=model,
        dimension=dimension,
        vector=vector,
    )


def test_identical_vectors_are_maximally_similar() -> None:
    a = _embedding((1.0, 0.0, 1.0))
    assert cosine_similarity(a, a) == pytest.approx(1.0)


def test_orthogonal_vectors_are_unrelated() -> None:
    assert cosine_similarity(_embedding((1.0, 0.0)), _embedding((0.0, 1.0))) == pytest.approx(0.0)


def test_a_zero_vector_does_not_divide_by_zero() -> None:
    assert cosine_similarity(_embedding((0.0, 0.0)), _embedding((1.0, 1.0))) == 0.0


def test_comparing_different_models_raises_rather_than_scoring() -> None:
    new = _embedding((0.1, 0.2))
    old = _embedding((0.1, 0.2), model="voyage-3", dimension=1024)

    with pytest.raises(IncomparableEmbeddings):
        cosine_similarity(new, old)


def test_comparing_the_same_model_at_two_widths_raises() -> None:
    narrow = _embedding((0.1, 0.2), dimension=512)
    wide = _embedding((0.1, 0.2), dimension=1536)

    with pytest.raises(IncomparableEmbeddings):
        cosine_similarity(narrow, wide)
