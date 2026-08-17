"""Embedding — a pure value type, no I/O.

The read side's own concept; there is no Go counterpart (embeddings never
enter the shared insights table, see IPP-97 / services/ai/README.md).
`model` + `dimension` travel with the vector itself, not just the table
schema, so a future model change is detectable instead of silently mixing
incompatible vector spaces.
"""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class Embedding:
    insight_id: str
    tenant_id: str
    model: str
    dimension: int
    vector: tuple[float, ...]
