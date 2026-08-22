"""OpenAI-backed EmbeddingClient, RelationLabeler, and ActionGenerator.

Replaces the Voyage adapter (IPP-135). Voyage existed only because
Anthropic served no embeddings; now that one provider covers enrichment
and embeddings both, this service and the Go worker share a single key —
see docs/adr/018-one-provider-for-model-capabilities.md.

Satisfies ipp_ai.ports.EmbeddingClient / RelationLabeler / ActionGenerator structurally;
this module does not import ipp_ai.ports (see ADR-017). Still plain
`urllib.request` rather than the `openai` SDK: one POST, one response,
and the SDK would be a dependency carried into the Lambda image for a
single call.

Bounded the same way internal/adapters/outbound/openai.Client is (ADR-013's
discipline, applied to a second capability): a timeout per attempt, capped
retries, and input truncated before it's sent. The embedding bound is on
total wall-clock time across every attempt, not per attempt, because it
has to fit inside the ai Lambda's own 30s timeout (terraform/envs/dev/ai.tf)
alongside the DynamoDB calls either side of it.
"""

from __future__ import annotations

import json
import logging
import time
import urllib.error
import urllib.request

from ipp_ai.domain.action import Action
from ipp_ai.domain.insight import Insight
from ipp_ai.domain.plan_context import ContextEdge
from ipp_ai.domain.relationship import RelationJudgement, RelationType

logger = logging.getLogger(__name__)

_EMBEDDINGS_ENDPOINT = "https://api.openai.com/v1/embeddings"
_CHAT_ENDPOINT = "https://api.openai.com/v1/chat/completions"
_MODEL = "text-embedding-3-small"

# Not the model's 1536 default. This system compares vectors by brute-force
# arithmetic with no vector index, and stores them as DynamoDB
# Decimal lists — so width sets item size, read-capacity cost and
# per-comparison cost at once. text-embedding-3-* are trained so a shortened
# vector keeps its concept-representing properties, which makes 512 a ~3x
# saving on all three for a corpus this size rather than a quality tax.
# Revisit if recall on real queries disappoints; it is a re-embed, not a
# schema change, because `dimension` travels with every stored vector.
_DIMENSION = 512

_TIMEOUT_SECONDS = 8
_MAX_ATTEMPTS = 3

# TRADE-OFF: character count as a stand-in for token count — no tokenizer
# dependency for one truncation guard. text-embedding-3-small's real limit
# is 8191 tokens, so this cuts in well before that; swap in a real tokenizer
# if truncation ever visibly clips meaningful input.
_MAX_INPUT_CHARS = 8000

# Same cheap-model judgement internal/adapters/outbound/openai.Client's
# enrichModel makes: labeling one pair is a bounded classification task,
# not a job that needs a frontier model.
_RELATION_MODEL = "gpt-5.6-luna"
_RELATION_MAX_TOKENS = 300
_RELATION_TIMEOUT_SECONDS = 15
_RELATION_SCHEMA_NAME = "label_relationship"

_RELATION_SYSTEM_PROMPT = (
    "You are labeling the relationship between two reading highlights from "
    "the same person's personal knowledge base. Given highlight A (existing) "
    "and highlight B (candidate), decide how B relates to A: does it support "
    "the same claim, contradict it, extend it with more depth, give a "
    "concrete example of it, or merely share the same topic without a "
    "stronger link? Pick exactly one relation_type. Give a confidence in "
    "[0, 1] reflecting how sure you are, and a one-to-two sentence rationale "
    "in plain language explaining the link. Be direct and concise. No "
    "preamble, no filler."
)

# relation_type's enum mirrors domain.relationship.RelationType exactly —
# `strict` structured outputs constrains the model to one of these values
# at generation time, the same substitution
# internal/adapters/outbound/openai.Client made for the Anthropic adapter's
# forced tool choice (IPP-135).
_RELATION_SCHEMA = {
    "type": "object",
    "properties": {
        "relation_type": {
            "type": "string",
            "enum": ["supports", "contradicts", "extends", "example_of", "same_topic"],
            "description": "How highlight B relates to highlight A.",
        },
        "confidence": {
            "type": "number",
            "description": "Confidence in [0, 1] that relation_type is correct.",
        },
        "rationale": {
            "type": "string",
            "description": "One or two plain-language sentences explaining the relation.",
        },
    },
    "required": ["relation_type", "confidence", "rationale"],
    "additionalProperties": False,
}


# Same cheap-model judgement as _RELATION_MODEL: drafting a handful of
# actions from an already-bounded context (PLAN 2 caps that at
# CONTEXT_TOKEN_BUDGET) is a constrained extraction task, not a job that
# needs a frontier model.
_ACTION_MODEL = "gpt-5.6-luna"
_ACTION_MAX_TOKENS = 800
_ACTION_TIMEOUT_SECONDS = 20
_ACTION_SCHEMA_NAME = "draft_weekly_actions"

_ACTION_SYSTEM_PROMPT = (
    "You are a weekly planning assistant. Given someone's stated focus for "
    "the week and a set of highlights from their reading (with any known "
    "relationships between them), propose 3-5 actions they can actually "
    "execute this week. Every action must be concrete and executable-this-"
    "week: a specific step, not vague advice. Reject anything that reads "
    "like 'reflect on X', 'think about Y', or 'read more about Z' — those "
    "are not actions. Center every action on the stated focus; do not "
    "drift into unrelated advice the highlights merely touch on. Cite the "
    "ids of only the highlights you were given that actually support each "
    "action — never invent an id. Be direct and concise. No preamble, no "
    "filler."
)

# One object per drafted action, mirroring _RELATION_SCHEMA's discipline:
# `strict` + `additionalProperties: False` + every property in `required`
# constrains the model to exactly this shape at generation time — the same
# substitution for a forced tool call the Go enrichment adapter documents
# (openai.go). supporting_insight_ids is deliberately still unvalidated
# here: the schema can force the *shape* of a citation, not whether the id
# is real, so application/action_generation.py checks that afterward.
_ACTION_SCHEMA = {
    "type": "object",
    "properties": {
        "actions": {
            "type": "array",
            "minItems": 3,
            "maxItems": 5,
            "items": {
                "type": "object",
                "properties": {
                    "title": {
                        "type": "string",
                        "description": "A single concrete, executable-this-week action.",
                    },
                    "why": {
                        "type": "string",
                        "description": (
                            "One or two plain-language sentences tying this action to the "
                            "user's stated focus and the highlights it cites."
                        ),
                    },
                    "supporting_insight_ids": {
                        "type": "array",
                        "items": {"type": "string"},
                        "minItems": 1,
                        "description": (
                            "ids, from the highlights supplied, that justify this action."
                        ),
                    },
                },
                "required": ["title", "why", "supporting_insight_ids"],
                "additionalProperties": False,
            },
        }
    },
    "required": ["actions"],
    "additionalProperties": False,
}


class OpenAiEmbeddingClient:
    """Embeds text via OpenAI's `/v1/embeddings` endpoint."""

    model = _MODEL
    dimension = _DIMENSION

    def __init__(self, api_key: str) -> None:
        self._api_key = api_key

    def embed(self, text: str) -> tuple[float, ...]:
        body = json.dumps(
            {
                "input": [text[:_MAX_INPUT_CHARS]],
                "model": self.model,
                "dimensions": self.dimension,
            }
        ).encode()
        request = urllib.request.Request(
            _EMBEDDINGS_ENDPOINT,
            data=body,
            method="POST",
            headers={
                "Authorization": f"Bearer {self._api_key}",
                "Content-Type": "application/json",
            },
        )
        payload = _send_json(request, timeout=_TIMEOUT_SECONDS)
        return tuple(payload["data"][0]["embedding"])


class OpenAiRelationLabeler:
    """Labels a candidate pair via OpenAI's `/v1/chat/completions`
    endpoint, one call per pair. `RelationType(...)` below is the
    belt-and-suspenders check for a response the `strict` schema should
    already guarantee — IPP-99's "rejected, not coerced" requirement,
    satisfied twice over rather than trusted to the API alone.
    """

    def __init__(self, api_key: str) -> None:
        self._api_key = api_key

    def label(self, from_text: str, to_text: str) -> RelationJudgement:
        body = json.dumps(
            {
                "model": _RELATION_MODEL,
                "max_completion_tokens": _RELATION_MAX_TOKENS,
                "messages": [
                    {"role": "system", "content": _RELATION_SYSTEM_PROMPT},
                    {
                        "role": "user",
                        "content": f"Highlight A:\n{from_text}\n\nHighlight B:\n{to_text}",
                    },
                ],
                "response_format": {
                    "type": "json_schema",
                    "json_schema": {
                        "name": _RELATION_SCHEMA_NAME,
                        "strict": True,
                        "schema": _RELATION_SCHEMA,
                    },
                },
            }
        ).encode()
        request = urllib.request.Request(
            _CHAT_ENDPOINT,
            data=body,
            method="POST",
            headers={
                "Authorization": f"Bearer {self._api_key}",
                "Content-Type": "application/json",
            },
        )

        start = time.monotonic()
        payload = _send_json(request, timeout=_RELATION_TIMEOUT_SECONDS)
        duration_ms = int((time.monotonic() - start) * 1000)

        # Field names match internal/adapters/outbound/openai.Client's
        # slog call — one Logs Insights query spans both services (IPP-113).
        usage = payload.get("usage", {})
        logger.info(
            "llm relation label complete",
            extra={
                "model": payload.get("model"),
                "input_tokens": usage.get("prompt_tokens"),
                "output_tokens": usage.get("completion_tokens"),
                "duration_ms": duration_ms,
            },
        )

        content = json.loads(payload["choices"][0]["message"]["content"])
        return RelationJudgement(
            relation_type=RelationType(content["relation_type"]),
            confidence=float(content["confidence"]),
            rationale=content["rationale"],
        )


class OpenAiActionGenerator:
    """Drafts weekly actions via OpenAI's `/v1/chat/completions` endpoint,
    one call per plan. Returns drafts as-is — `Action.supporting_insight_ids`
    is exactly what the model claimed; PLAN 3's citation validation
    (application/action_generation.py) is what turns "claimed" into
    "verified", deliberately kept out of this adapter.
    """

    def __init__(self, api_key: str) -> None:
        self._api_key = api_key

    def generate(
        self, focus_sentence: str, insights: list[Insight], edges: tuple[ContextEdge, ...]
    ) -> list[Action]:
        body = json.dumps(
            {
                "model": _ACTION_MODEL,
                "max_completion_tokens": _ACTION_MAX_TOKENS,
                "messages": [
                    {"role": "system", "content": _ACTION_SYSTEM_PROMPT},
                    {
                        "role": "user",
                        "content": _format_action_context(focus_sentence, insights, edges),
                    },
                ],
                "response_format": {
                    "type": "json_schema",
                    "json_schema": {
                        "name": _ACTION_SCHEMA_NAME,
                        "strict": True,
                        "schema": _ACTION_SCHEMA,
                    },
                },
            }
        ).encode()
        request = urllib.request.Request(
            _CHAT_ENDPOINT,
            data=body,
            method="POST",
            headers={
                "Authorization": f"Bearer {self._api_key}",
                "Content-Type": "application/json",
            },
        )

        start = time.monotonic()
        payload = _send_json(request, timeout=_ACTION_TIMEOUT_SECONDS)
        duration_ms = int((time.monotonic() - start) * 1000)

        usage = payload.get("usage", {})
        logger.info(
            "llm weekly plan action draft complete",
            extra={
                "model": payload.get("model"),
                "input_tokens": usage.get("prompt_tokens"),
                "output_tokens": usage.get("completion_tokens"),
                "duration_ms": duration_ms,
            },
        )

        content = json.loads(payload["choices"][0]["message"]["content"])
        return [
            Action(
                title=raw["title"],
                why=raw["why"],
                supporting_insight_ids=tuple(raw["supporting_insight_ids"]),
            )
            for raw in content["actions"]
        ]


def _format_action_context(
    focus_sentence: str, insights: list[Insight], edges: tuple[ContextEdge, ...]
) -> str:
    lines = [f"This week's focus: {focus_sentence}", "", "Highlights:"]
    for insight in insights:
        lines.append(f"- id: {insight.id}\n  text: {insight.text}")

    if edges:
        lines.append("\nRelationships between highlights:")
        for edge in edges:
            lines.append(
                f"- {edge.from_insight_id} {edge.relation_type} {edge.to_insight_id}: "
                f"{edge.rationale}"
            )

    return "\n".join(lines)


def _send_json(request: urllib.request.Request, *, timeout: float) -> dict:
    # _MAX_ATTEMPTS >= 1 guarantees this placeholder is always overwritten below.
    last_error: Exception = RuntimeError("unreachable")
    for _ in range(_MAX_ATTEMPTS):
        try:
            with urllib.request.urlopen(request, timeout=timeout) as response:
                return json.load(response)
        except urllib.error.HTTPError as exc:
            if exc.code < 500:
                raise  # bad key / bad request — retrying changes nothing
            last_error = exc
        except (urllib.error.URLError, TimeoutError) as exc:
            last_error = exc
    raise last_error
