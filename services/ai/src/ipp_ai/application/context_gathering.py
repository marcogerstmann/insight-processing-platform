"""Assembles the Action Agent's bounded context — PLAN 2/IPP-104.

Sits after WeeklyPlanRequested the same way application/relationship.py's
discover_relationships sits after InsightEnriched: this is what the Lambda
handler actually calls, wiring domain/plan_context.py's pure selection to the
adapters that load a tag's insights (TAG 4) and their relationships (REL 6).

No LLM call here — PLAN 3 is the next story and will call this function
(or its result) before making one. This slice's entire job is producing a
bounded, reconstructable context or a clear "not enough material" outcome.
"""

from __future__ import annotations

import logging
from datetime import datetime

from ipp_ai.domain.plan_context import InsufficientMaterial, SelectedContext, select_context
from ipp_ai.ports import InsightReader, RelationshipReader

logger = logging.getLogger(__name__)


def gather_context(
    tenant_id: str,
    tag: str,
    *,
    insight_reader: InsightReader,
    relationship_reader: RelationshipReader,
    now: datetime,
) -> SelectedContext | InsufficientMaterial:
    """Loads `tag`'s insights and every one of their relationship edges, then
    delegates to select_context for the deterministic cut.

    One relationship query per insight in the tag, mirroring ListTags'
    aggregation (internal/adapters/outbound/dynamodb.InsightAdapter.ListTags):
    fine at a personal knowledge base's scale (a tag rarely holds more than a
    few hundred insights); revisit if a real tag ever makes this the
    bottleneck.
    """
    insights = insight_reader.list_by_tag(tenant_id, tag)
    relationships = {
        insight.id: relationship_reader.list_by_insight(tenant_id, insight.id)
        for insight in insights
    }

    context = select_context(tag, insights, relationships, now=now)
    _log_outcome(tenant_id, context)
    return context


def _log_outcome(tenant_id: str, context: SelectedContext | InsufficientMaterial) -> None:
    if isinstance(context, InsufficientMaterial):
        logger.info(
            "not enough material to plan a week",
            extra={
                "tenant_id": tenant_id,
                "tag": context.tag,
                "insight_count": context.insight_count,
            },
        )
        return

    # Selected insight ids + budget used, logged so the input to any plan is
    # reconstructable after the fact (IPP-104's AC) — even though nothing
    # downstream persists this context yet.
    logger.info(
        "selected weekly plan context",
        extra={
            "tenant_id": tenant_id,
            "tag": context.tag,
            "insight_ids": [i.id for i in context.insights],
            "edge_count": len(context.edges),
            "estimated_tokens": context.estimated_tokens,
        },
    )
