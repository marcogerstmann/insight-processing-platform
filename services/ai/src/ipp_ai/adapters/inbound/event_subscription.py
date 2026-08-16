"""EventBridge domain-event subscription handler.

The read side's counterpart to internal/adapters/inbound/sqs/worker.Handler.
EventBridge delivers events matching this service's rule
(terraform/envs/dev/ai.tf, built on terraform/modules/event-subscription) to
its own SQS queue; each record's body is the full EventBridge envelope, and
"detail" is the JSON of internal/domain.DomainEvent.

This does the minimum IPP-95 asks for: unmarshal, log structurally, done —
no repository call, no business logic. That arrives with whatever agent
story first needs one.

`Handler` wires nothing itself (only depends on the DlqPublisher port, so
it's testable without AWS); `lambda_handler` is the composition root that
constructs the real adapter and is what the Dockerfile's CMD points at —
per ADR-017, Python's handler does its own manual DI instead of a separate
main.go.
"""

from __future__ import annotations

import json
import logging
import os
from datetime import datetime
from typing import Any

from ipp_ai.adapters.outbound.sqs import SqsDlqPublisher
from ipp_ai.domain.event import DomainEvent
from ipp_ai.errors import PermanentError
from ipp_ai.logging_config import configure
from ipp_ai.ports import DlqPublisher

configure()
logger = logging.getLogger(__name__)


class Handler:
    """Permanent unmarshal failure -> DLQ + continue; everything else
    propagates so the runtime redelivers (ADR-009's taxonomy, translated).

    Safe as a per-record loop only because the event source mapping in
    terraform/envs/dev/ai.tf keeps batch_size = 1 — same constraint as the
    Go worker; raising it needs ReportBatchItemFailures first.
    """

    def __init__(self, dlq: DlqPublisher) -> None:
        self._dlq = dlq

    def handle(self, event: dict[str, Any]) -> None:
        for record in event["Records"]:
            body = record["body"]
            try:
                domain_event = _parse_envelope(body)
            except PermanentError as exc:
                self._route_to_dlq(body, exc)
                continue
            _log_event(domain_event)

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
    Handler(dlq).handle(event)
