"""Embedding — a pure value type, no I/O.

The read side's own concept; there is no Go counterpart (embeddings never
enter the shared insights table, see IPP-97 / services/ai/README.md).
`model` + `dimension` travel with the vector itself, not just the table
schema, so a future model change is detectable instead of silently mixing
incompatible vector spaces.
"""

from __future__ import annotations

import math
from dataclasses import dataclass


@dataclass(frozen=True)
class Embedding:
    insight_id: str
    tenant_id: str
    model: str
    dimension: int
    vector: tuple[float, ...]


class IncomparableEmbeddings(Exception):
    """Two embeddings from different vector spaces were compared."""


def cosine_similarity(a: Embedding, b: Embedding) -> float:
    """Similarity in [-1, 1], or a raise if the two are not comparable.

    The guard is the point of this function, not the arithmetic. Vectors
    from two different models — or the same model at two widths — occupy
    unrelated spaces, so a cosine over them returns a plausible-looking
    number and no error anywhere. That is the failure mode IPP-135's
    provider switch could have introduced silently, and the reason IPP-97
    put `model` and `dimension` on every stored item in the first place.

    Brute force on purpose: no index, no vector database. At this corpus
    size the whole cost is one pass per candidate.
    """
    if a.model != b.model or a.dimension != b.dimension:
        raise IncomparableEmbeddings(
            f"cannot compare {a.model}/{a.dimension} with {b.model}/{b.dimension} — "
            "different vector spaces; re-embed one side before comparing"
        )

    dot = sum(x * y for x, y in zip(a.vector, b.vector, strict=True))
    magnitude = math.sqrt(sum(x * x for x in a.vector)) * math.sqrt(sum(y * y for y in b.vector))
    return dot / magnitude if magnitude else 0.0
