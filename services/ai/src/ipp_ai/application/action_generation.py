"""Generates and validates the Action Agent's weekly actions — PLAN 3/IPP-105.

Sits after application/context_gathering.py's gather_context: that function
produces a bounded, reconstructable SelectedContext; this spends the one LLM
call PLAN 2 exists to bound the cost of, then throws away anything the model
claims that context didn't actually contain.

The citation check is the point of this story, not a nicety bolted onto it:
a schema-constrained model still free-associates on the *values* inside that
schema, and a plan the user can't trust as grounded is worse than no plan.
Every supporting_insight_id is checked against the ids SelectedContext
actually handed the model; a hallucinated id is dropped and logged, and an
action left with no surviving id is discarded outright. Fewer than 3 valid
actions is a legitimate outcome (IPP-105's AC) — never padded to a target
count, the same "no plan is better than a bad plan" posture
InsufficientMaterial takes one step upstream.
"""

from __future__ import annotations

import logging

from ipp_ai.domain.action import Action
from ipp_ai.domain.plan_context import SelectedContext
from ipp_ai.ports import ActionGenerator

logger = logging.getLogger(__name__)


def generate_actions(
    tenant_id: str,
    focus_sentence: str,
    context: SelectedContext,
    *,
    generator: ActionGenerator,
) -> list[Action]:
    """Drafts actions for `context`, then keeps only the ones with at least
    one citation `context.insights` actually contains.

    Never raises for a bad *shape* of model response — an empty draft list
    resolves to an empty result, same as label_relationships' "no
    relationship found" outcome. A transport/timeout failure from
    `generator` still propagates: retries and the token/timeout bound are
    the adapter's job (same as RelationLabeler), not something this
    function papers over.
    """
    valid_ids = {insight.id for insight in context.insights}
    drafts = generator.generate(focus_sentence, list(context.insights), context.edges)

    actions: list[Action] = []
    for draft in drafts:
        surviving = tuple(i for i in draft.supporting_insight_ids if i in valid_ids)
        hallucinated = [i for i in draft.supporting_insight_ids if i not in valid_ids]
        if hallucinated:
            logger.warning(
                "dropped hallucinated supporting_insight_id(s)",
                extra={
                    "tenant_id": tenant_id,
                    "tag": context.tag,
                    "title": draft.title,
                    "dropped_ids": hallucinated,
                },
            )

        if not surviving:
            logger.info(
                "discarded action with no surviving supporting insight",
                extra={"tenant_id": tenant_id, "tag": context.tag, "title": draft.title},
            )
            continue

        actions.append(Action(title=draft.title, why=draft.why, supporting_insight_ids=surviving))

    logger.info(
        "generated weekly plan actions",
        extra={
            "tenant_id": tenant_id,
            "tag": context.tag,
            "drafted_count": len(drafts),
            "valid_count": len(actions),
        },
    )
    return actions
