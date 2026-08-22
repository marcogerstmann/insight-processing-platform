"""EventBridge domain-event subscription handler.

The read side's counterpart to internal/adapters/inbound/sqs/worker.Handler.
EventBridge delivers events matching this service's rule
(terraform/envs/dev/ai.tf, built on terraform/modules/event-subscription) to
its own SQS queue; each record's body is the full EventBridge envelope, and
"detail" is the JSON of internal/domain.DomainEvent.

IPP-95 did the minimum: unmarshal, log structurally, done. IPP-97 adds
embedding (`application/embedding.py`), and REL 2-4 (IPP-98/99/100) add
relationship discovery (`application/relationship.py`) straight after it —
same event, same record, same DLQ-or-propagate branch. Those three tickets
shipped as pure, independently-tested functions; this handler is what
actually calls them in production.

PLAN 2 (IPP-104) adds a second event type on its own subscription queue
(`terraform/envs/dev/ai.tf`'s `action_agent_subscription`, wired to this same
Lambda by a second event source mapping): `WeeklyPlanRequested` branches to
`application/context_gathering.py`'s `gather_context` instead of the
embed+relate pipeline above — a different event type, a different job, same
handler and same DLQ-or-propagate branch either way.

PLAN 3 (IPP-105) extends that branch: once `gather_context` returns a
`SelectedContext` (not `InsufficientMaterial`, which has nothing to plan
from), `application/action_generation.py`'s `generate_actions` makes the
one LLM call that drafts and citation-validates the week's actions. PLAN 4
(IPP-106) persists the result via `PlanResultWriter` — `set_ready` for a
`SelectedContext` outcome, `set_failed` for `InsufficientMaterial`. PLAN 5
(IPP-107) is what will make that write survive redelivery; until then a
retried event can call `set_ready`/`set_failed` a second time and get the
Go API's 409 (the plan is no longer pending), same as a genuinely duplicate
delivery.

`Handler` wires nothing itself (only depends on the ports its constructor
takes, so it's testable without AWS); `lambda_handler` is the composition
root that constructs the real adapters and is what the Dockerfile's CMD
points at — per ADR-017, Python's handler does its own manual DI instead of
a separate main.go.
"""

from __future__ import annotations

import json
import logging
import os
from datetime import UTC, datetime
from typing import Any

from ipp_ai.adapters.outbound.cognito import CognitoServiceTokenClient
from ipp_ai.adapters.outbound.dynamodb import DynamoDbInsightReader
from ipp_ai.adapters.outbound.embedding_store import DynamoDbEmbeddingWriter
from ipp_ai.adapters.outbound.openai import (
    OpenAiActionGenerator,
    OpenAiEmbeddingClient,
    OpenAiRelationLabeler,
)
from ipp_ai.adapters.outbound.plan_result_api import GoApiPlanResultWriter
from ipp_ai.adapters.outbound.relationship_api import GoApiRelationshipWriter
from ipp_ai.adapters.outbound.sqs import SqsDlqPublisher
from ipp_ai.adapters.outbound.ssm import SsmSecretProvider
from ipp_ai.application.action_generation import generate_actions
from ipp_ai.application.context_gathering import gather_context
from ipp_ai.application.embedding import embed_insight
from ipp_ai.application.relationship import discover_relationships
from ipp_ai.application.secrets import resolve_secret
from ipp_ai.domain.event import DomainEvent
from ipp_ai.domain.plan_context import InsufficientMaterial
from ipp_ai.errors import PermanentError
from ipp_ai.logging_config import configure
from ipp_ai.ports import (
    ActionGenerator,
    DlqPublisher,
    EmbeddingClient,
    EmbeddingReader,
    EmbeddingWriter,
    InsightReader,
    PlanResultWriter,
    RelationLabeler,
    RelationshipReader,
    RelationshipWriter,
)

configure()
logger = logging.getLogger(__name__)


class Handler:
    """Permanent failure (malformed envelope, unknown insight) -> DLQ +
    continue; everything else propagates so the runtime redelivers
    (ADR-009's taxonomy, translated).

    Safe as a per-record loop only because the event source mapping in
    terraform/envs/dev/ai.tf keeps batch_size = 1 — same constraint as the
    Go worker; raising it needs ReportBatchItemFailures first.
    """

    def __init__(
        self,
        dlq: DlqPublisher,
        reader: InsightReader,
        embedder: EmbeddingClient,
        writer: EmbeddingWriter,
        embedding_reader: EmbeddingReader,
        labeler: RelationLabeler,
        relationship_writer: RelationshipWriter,
        relationship_reader: RelationshipReader,
        action_generator: ActionGenerator,
        plan_result_writer: PlanResultWriter,
    ) -> None:
        self._dlq = dlq
        self._reader = reader
        self._embedder = embedder
        self._writer = writer
        self._embedding_reader = embedding_reader
        self._labeler = labeler
        self._relationship_writer = relationship_writer
        self._relationship_reader = relationship_reader
        self._action_generator = action_generator
        self._plan_result_writer = plan_result_writer

    def handle(self, event: dict[str, Any]) -> None:
        for record in event["Records"]:
            body = record["body"]
            try:
                domain_event = _parse_envelope(body)
                _log_event(domain_event)

                if domain_event.event_type == "WeeklyPlanRequested":
                    plan_id = domain_event.payload["plan_id"]
                    context = gather_context(
                        domain_event.tenant_id,
                        domain_event.payload.get("tag"),
                        insight_reader=self._reader,
                        relationship_reader=self._relationship_reader,
                        now=datetime.now(UTC),
                    )
                    if isinstance(context, InsufficientMaterial):
                        self._plan_result_writer.set_failed(
                            domain_event.tenant_id,
                            plan_id,
                            f'not enough insights tagged "{context.tag}" to plan a week '
                            f"(found {context.insight_count})",
                        )
                        continue

                    actions = generate_actions(
                        domain_event.tenant_id,
                        domain_event.payload.get("focus_sentence", ""),
                        context,
                        generator=self._action_generator,
                    )
                    self._plan_result_writer.set_ready(domain_event.tenant_id, plan_id, actions)
                    continue

                embedding = embed_insight(
                    domain_event.tenant_id,
                    domain_event.payload.get("insight_id"),
                    reader=self._reader,
                    embedder=self._embedder,
                    writer=self._writer,
                )
                discover_relationships(
                    domain_event.tenant_id,
                    embedding,
                    insight_reader=self._reader,
                    embedding_reader=self._embedding_reader,
                    labeler=self._labeler,
                    relationship_writer=self._relationship_writer,
                )
            except PermanentError as exc:
                self._route_to_dlq(body, exc)
                continue

    def _route_to_dlq(self, body: str, reason: PermanentError) -> None:
        logger.error("permanent error, routed to DLQ", extra={"err": str(reason)})
        try:
            self._dlq.send(body, str(reason))
        except Exception:
            logger.exception("failed to send message to DLQ")


def _parse_envelope(body: str) -> DomainEvent:
    try:
        detail = json.loads(body)["detail"]
        return DomainEvent(
            event_id=detail["event_id"],
            event_type=detail["event_type"],
            version=detail["version"],
            tenant_id=detail["tenant_id"],
            occurred_at=datetime.fromisoformat(detail["occurred_at"]),
            payload=detail.get("payload") or {},
        )
    except (json.JSONDecodeError, KeyError, TypeError, ValueError) as exc:
        raise PermanentError(f"malformed EventBridge envelope: {exc}") from exc


def _log_event(event: DomainEvent) -> None:
    logger.info(
        "received domain event",
        extra={
            "tenant_id": event.tenant_id,
            "insight_id": event.payload.get("insight_id"),
            "event_type": event.event_type,
        },
    )


def lambda_handler(event: dict[str, Any], _context: Any) -> None:
    dlq = SqsDlqPublisher(os.environ["AI_SUBSCRIPTION_DLQ_URL"])
    reader = DynamoDbInsightReader(os.environ["TABLE_NAME_INSIGHTS"])
    # One adapter instance serves both EmbeddingWriter (embed_insight) and
    # EmbeddingReader (REL 2's candidate pool) — same table, same class.
    embeddings = DynamoDbEmbeddingWriter(os.environ["TABLE_NAME_EMBEDDINGS"])

    api_key = resolve_secret("OPENAI_API_KEY", SsmSecretProvider())
    if not api_key:
        raise RuntimeError("OPENAI_API_KEY not configured")
    embedder = OpenAiEmbeddingClient(api_key)
    labeler = OpenAiRelationLabeler(api_key)
    action_generator = OpenAiActionGenerator(api_key)

    agent_secret = resolve_secret("AGENT_CLIENT_SECRET", SsmSecretProvider())
    token_client = CognitoServiceTokenClient(
        token_endpoint=os.environ["AGENT_TOKEN_ENDPOINT"],
        client_id=os.environ["AGENT_CLIENT_ID"],
        client_secret=agent_secret,
        scope=os.environ.get("AGENT_SCOPE", "ipp/agent.write"),
    )
    relationship_writer = GoApiRelationshipWriter(os.environ["REST_API_BASE_URL"], token_client)
    # Shares token_client with relationship_writer above: both calls need the
    # same agent.write-scoped machine token, and CognitoServiceTokenClient
    # already caches it (see its own module) rather than re-authenticating
    # per call.
    plan_result_writer = GoApiPlanResultWriter(os.environ["REST_API_BASE_URL"], token_client)

    # Same DynamoDbInsightReader instance also satisfies RelationshipReader
    # (REL 6) — one class, one table, same read-only boundary as InsightReader.
    handler = Handler(
        dlq,
        reader,
        embedder,
        embeddings,
        embeddings,
        labeler,
        relationship_writer,
        reader,
        action_generator,
        plan_result_writer,
    )
    handler.handle(event)
