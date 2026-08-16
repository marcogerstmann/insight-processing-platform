"""Structured JSON logging — the Python counterpart to internal/logging.

One JSON object per log line, honoring LOG_LEVEL the same way
internal/logging.resolveLevel does. Call configure() once, at import time
of a Lambda entrypoint (see adapters/inbound/event_subscription.py) — a
cold start runs module-level code exactly once per container.
"""

from __future__ import annotations

import json
import logging
import os
from datetime import UTC, datetime

# Every attribute a bare LogRecord carries by default, so JsonFormatter can
# tell those apart from whatever a caller passed via `extra=`.
_RESERVED_ATTRS = frozenset(logging.LogRecord("", 0, "", 0, "", (), None).__dict__) | {"message"}


class JsonFormatter(logging.Formatter):
    """Renders a LogRecord as one JSON object. Field names for anything
    passed via `extra=` match what the Go service's slog.NewJSONHandler
    logs (tenant_id, insight_id, event_type, ...).
    """

    def format(self, record: logging.LogRecord) -> str:
        payload = {
            "time": datetime.fromtimestamp(record.created, tz=UTC).isoformat(),
            "level": record.levelname.lower(),
            "msg": record.getMessage(),
        }
        payload.update((k, v) for k, v in record.__dict__.items() if k not in _RESERVED_ATTRS)
        if record.exc_info:
            payload["err"] = self.formatException(record.exc_info)
        return json.dumps(payload, default=str)


def configure(level: str | None = None) -> None:
    """Install the JSON formatter on the root logger."""
    name = (level or os.environ.get("LOG_LEVEL", "INFO")).upper()
    resolved = getattr(logging, name, logging.INFO)

    handler = logging.StreamHandler()
    handler.setFormatter(JsonFormatter())

    root = logging.getLogger()
    root.handlers = [handler]
    root.setLevel(resolved)
