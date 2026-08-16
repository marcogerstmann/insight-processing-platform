"""boto3-backed InsightReader — the read-only AWS counterpart to
internal/adapters/outbound/dynamodb.InsightAdapter.

Satisfies ipp_ai.ports.InsightReader structurally; it does not import
ipp_ai.ports (see ADR-017). Key schema mirrors the Go adapter exactly —
pk = TENANT#<tenant_id>, sk = INSIGHT#<id>; tag membership lives in the
same partition (sk = TAG#<tag>#INSIGHT#<id>) and is queryable via the
sparse gsi1 index. Two languages now unmarshal the same items: a schema
change is a two-repo-location change.

Read-only by construction: GetItem and Query are the only calls this file
makes. The Lambda execution role wired up in IPP-95 must not grant this
service PutItem/UpdateItem/DeleteItem — see services/ai/README.md.
"""

from __future__ import annotations

from datetime import datetime
from typing import Any

import boto3
from boto3.dynamodb.conditions import Key

from ipp_ai.domain.insight import Enrichment, Insight
from ipp_ai.errors import PermanentError

_TAG_INDEX_NAME = "gsi1"  # must match terraform/modules/dynamodb/main.tf


def _pk(tenant_id: str) -> str:
    return f"TENANT#{tenant_id}"


def _sk(insight_id: str) -> str:
    return f"INSIGHT#{insight_id}"


def _tag_sk_prefix(tag: str) -> str:
    return f"TAG#{tag}#INSIGHT#"


class DynamoDbInsightReader:
    """Read-only access to the insights table."""

    def __init__(self, table_name: str, resource: Any | None = None) -> None:
        self._table = (resource or boto3.resource("dynamodb")).Table(table_name)

    def get_by_id(self, tenant_id: str, insight_id: str) -> Insight | None:
        response = self._table.get_item(Key={"pk": _pk(tenant_id), "sk": _sk(insight_id)})
        item = response.get("Item")
        return _unmarshal_insight(item) if item is not None else None

    def list_by_tenant(self, tenant_id: str) -> list[Insight]:
        response = self._table.query(
            KeyConditionExpression=Key("pk").eq(_pk(tenant_id)) & Key("sk").begins_with("INSIGHT#"),
        )
        return [_unmarshal_insight(item) for item in response["Items"]]

    def list_by_tag(self, tenant_id: str, tag: str) -> list[Insight]:
        response = self._table.query(
            IndexName=_TAG_INDEX_NAME,
            KeyConditionExpression=Key("gsi1pk").eq(_pk(tenant_id))
            & Key("gsi1sk").begins_with(_tag_sk_prefix(tag)),
        )
        insights = []
        for membership in response["Items"]:
            insight = self.get_by_id(tenant_id, membership["insight_id"])
            if insight is not None:
                insights.append(insight)
            # else: orphaned membership (insight deleted after tagging); skip.
        return insights


def _unmarshal_insight(item: dict[str, Any]) -> Insight:
    try:
        enrichment_item = item.get("enrichment")
        enrichment = Enrichment(tags=tuple(enrichment_item["tags"])) if enrichment_item else None
        return Insight(
            id=item["id"],
            tenant_id=item["tenant_id"],
            source=item["source"],
            text=item["text"],
            notes=item["notes"],
            highlighted_at=datetime.fromisoformat(item["highlighted_at"]),
            enrichment=enrichment,
        )
    except (KeyError, ValueError, TypeError) as exc:
        raise PermanentError(
            f"malformed insight item: pk={item.get('pk')!r} sk={item.get('sk')!r}"
        ) from exc
