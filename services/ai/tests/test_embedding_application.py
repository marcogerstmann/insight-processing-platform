from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime

import pytest

from ipp_ai.application.embedding import embed_insight
from ipp_ai.domain.insight import Enrichment, Insight
from ipp_ai.errors import PermanentError


@dataclass
class FakeReader:
    insights: dict[tuple[str, str], Insight] = field(default_factory=dict)

    def get_by_id(self, tenant_id: str, insight_id: str) -> Insight | None:
        return self.insights.get((tenant_id, insight_id))


@dataclass
class SpyEmbedder:
    model: str = "text-embedding-3-small"
    dimension: int = 2
    vector: tuple[float, ...] = (0.5, 0.6)
    received: list[str] = field(default_factory=list)

    def embed(self, text: str) -> tuple[float, ...]:
        self.received.append(text)
        return self.vector


@dataclass
class SpyWriter:
    puts: list[object] = field(default_factory=list)

    def put(self, embedding: object) -> None:
        self.puts.append(embedding)


def _insight(*, enrichment: Enrichment | None = None) -> Insight:
    return Insight(
        id="i1",
        tenant_id="t1",
        source="readwise",
        text="the highlight text",
        notes="",
        highlighted_at=datetime.fromisoformat("2026-01-01T00:00:00"),
        enrichment=enrichment,
    )


def test_embeds_tags_and_text_together_when_enriched() -> None:
    reader = FakeReader({("t1", "i1"): _insight(enrichment=Enrichment(tags=("work", "focus")))})
    embedder = SpyEmbedder()
    writer = SpyWriter()

    embed_insight("t1", "i1", reader=reader, embedder=embedder, writer=writer)

    assert embedder.received == ["work, focus\n\nthe highlight text"]
    assert writer.puts[0].vector == (0.5, 0.6)
    assert writer.puts[0].model == "text-embedding-3-small"
    assert writer.puts[0].dimension == 2


def test_embeds_text_alone_when_unenriched() -> None:
    reader = FakeReader({("t1", "i1"): _insight(enrichment=None)})
    embedder = SpyEmbedder()
    writer = SpyWriter()

    embed_insight("t1", "i1", reader=reader, embedder=embedder, writer=writer)

    assert embedder.received == ["the highlight text"]


def test_missing_insight_id_is_permanent_error() -> None:
    with pytest.raises(PermanentError):
        embed_insight("t1", None, reader=FakeReader(), embedder=SpyEmbedder(), writer=SpyWriter())


def test_unknown_insight_is_permanent_error() -> None:
    with pytest.raises(PermanentError):
        embed_insight(
            "t1", "missing", reader=FakeReader(), embedder=SpyEmbedder(), writer=SpyWriter()
        )
