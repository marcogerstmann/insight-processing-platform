"""Embeds an insight on InsightEnriched — IPP-97.

The read side's counterpart to internal/application/llm.Service, but
embedding is not optional the way ADR-013 makes enrichment optional: there
is no already-durable write this step is protecting, so failure is not
swallowed — it propagates (PermanentError aside) so the runtime redelivers
and, after this subscription's own retry budget, lands on its DLQ
(ADR-009). That satisfies IPP-97's "failure is soft ... never affects the
core pipeline": this is a subscriber off to the side of the Go core, so a
stuck or failing embedding call can never block an insight write.
"""

from __future__ import annotations

from ipp_ai.domain.embedding import Embedding
from ipp_ai.errors import PermanentError
from ipp_ai.ports import EmbeddingClient, EmbeddingWriter, InsightReader


def embed_insight(
    tenant_id: str,
    insight_id: str | None,
    *,
    reader: InsightReader,
    embedder: EmbeddingClient,
    writer: EmbeddingWriter,
) -> None:
    if not insight_id:
        raise PermanentError("InsightEnriched payload missing insight_id")

    insight = reader.get_by_id(tenant_id, insight_id)
    if insight is None:
        raise PermanentError(f"no insight for tenant={tenant_id} id={insight_id}")

    # "summary" per IPP-97's acceptance criteria: the tags enrichment
    # already distilled the highlight into, not a separate summary field —
    # Insight carries no such field, and generating one would be a second
    # LLM call this ticket never asks for.
    summary = ", ".join(insight.enrichment.tags) if insight.enrichment else ""
    text = f"{summary}\n\n{insight.text}" if summary else insight.text

    vector = embedder.embed(text)

    writer.put(
        Embedding(
            insight_id=insight_id,
            tenant_id=tenant_id,
            model=embedder.model,
            dimension=embedder.dimension,
            vector=vector,
        )
    )
