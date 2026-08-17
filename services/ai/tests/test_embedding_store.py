from __future__ import annotations

from decimal import Decimal

from ipp_ai.domain.embedding import Embedding


def test_put_writes_the_deterministic_key_and_decimal_vector(stubbed_writer) -> None:
    stubbed_writer.stubber.add_response(
        "put_item",
        {},
        {
            "TableName": "test-embeddings",
            "Item": {
                "pk": "TENANT#t1",
                "sk": "EMBEDDING#i1",
                "tenant_id": "t1",
                "insight_id": "i1",
                "model": "voyage-3",
                "dimension": 3,
                # DynamoDB has no float type — Decimal is what boto3's
                # resource layer requires on the wire.
                "vector": [Decimal("0.1"), Decimal("0.2"), Decimal("0.3")],
            },
        },
    )

    stubbed_writer.writer.put(
        Embedding(
            insight_id="i1",
            tenant_id="t1",
            model="voyage-3",
            dimension=3,
            vector=(0.1, 0.2, 0.3),
        )
    )


def test_put_is_idempotent_by_key(stubbed_writer) -> None:
    """Two puts for the same (tenant, insight) both hit the same key — no
    conditional expression, so the second silently overwrites the first
    rather than erroring or duplicating (IPP-97's idempotency requirement).
    """
    embedding = Embedding(
        insight_id="i1", tenant_id="t1", model="voyage-3", dimension=1, vector=(0.9,)
    )
    for _ in range(2):
        stubbed_writer.stubber.add_response(
            "put_item",
            {},
            {
                "TableName": "test-embeddings",
                "Item": {
                    "pk": "TENANT#t1",
                    "sk": "EMBEDDING#i1",
                    "tenant_id": "t1",
                    "insight_id": "i1",
                    "model": "voyage-3",
                    "dimension": 1,
                    "vector": [Decimal("0.9")],
                },
            },
        )

    stubbed_writer.writer.put(embedding)
    stubbed_writer.writer.put(embedding)
