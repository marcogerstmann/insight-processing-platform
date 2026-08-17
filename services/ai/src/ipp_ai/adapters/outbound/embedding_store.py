"""boto3-backed EmbeddingWriter — writes to the AI service's own DynamoDB
table (`terraform/envs/dev/ai.tf`'s dynamodb_ai_embeddings module), not the
shared insights table `dynamodb.py` reads from.

Satisfies ipp_ai.ports.EmbeddingWriter structurally; it does not import
ipp_ai.ports (see ADR-017). Writing here is fine even though the rest of
this service is read-only elsewhere (services/ai/README.md) — this table
belongs to this service alone, unlike the insights table the Go core owns.

Key schema per IPP-97: pk = TENANT#<tenant_id>, sk = EMBEDDING#<insightID>.
PutItem with no condition, so re-processing the same event overwrites in
place rather than duplicating — the idempotency IPP-97 asks for comes for
free from the deterministic key, the same way it does everywhere else in
this codebase (ADR-008).
"""

from __future__ import annotations

from decimal import Decimal
from typing import Any

import boto3

from ipp_ai.domain.embedding import Embedding


def _pk(tenant_id: str) -> str:
    return f"TENANT#{tenant_id}"


def _sk(insight_id: str) -> str:
    return f"EMBEDDING#{insight_id}"


class DynamoDbEmbeddingWriter:
    """Upserts an insight's embedding."""

    def __init__(self, table_name: str, resource: Any | None = None) -> None:
        self._table = (resource or boto3.resource("dynamodb")).Table(table_name)

    def put(self, embedding: Embedding) -> None:
        self._table.put_item(
            Item={
                "pk": _pk(embedding.tenant_id),
                "sk": _sk(embedding.insight_id),
                "tenant_id": embedding.tenant_id,
                "insight_id": embedding.insight_id,
                "model": embedding.model,
                "dimension": embedding.dimension,
                # DynamoDB's Number type has no native float — boto3's
                # resource layer requires Decimal, via str() to avoid
                # binary-float imprecision leaking into the stored value.
                "vector": [Decimal(str(v)) for v in embedding.vector],
            }
        )
