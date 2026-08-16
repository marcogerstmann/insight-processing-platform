"""DomainEvent — the read side's counterpart to internal/domain.DomainEvent.

Mirrors the envelope internal/adapters/outbound/eventbridge.DomainEventPublisher
publishes as an EventBridge entry's "detail" field. A pure value type only —
parsing untrusted JSON into it is adapter work, same as AI 2's insight
unmarshalling (see adapters/inbound/event_subscription.py).
"""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime
from typing import Any


@dataclass(frozen=True)
class DomainEvent:
    event_id: str
    event_type: str
    version: int
    tenant_id: str
    occurred_at: datetime
    payload: dict[str, Any] = field(default_factory=dict)
