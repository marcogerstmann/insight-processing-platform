from __future__ import annotations

import json
from dataclasses import dataclass, field

import pytest

from ipp_ai.adapters.inbound.event_subscription import Handler


@dataclass
class SpyDlq:
    sent: list[tuple[str, str]] = field(default_factory=list)
    send_error: Exception | None = None

    def send(self, body: str, reason: str) -> None:
        self.sent.append((body, reason))
        if self.send_error is not None:
            raise self.send_error


def _envelope(
    *, event_type: str = "InsightEnriched", tenant_id: str = "t1", insight_id: str = "i1"
) -> str:
    return json.dumps(
        {
            "version": "0",
            "id": "evt-1",
            "detail-type": event_type,
            "source": "ipp.core",
            "account": "123456789012",
            "time": "2026-08-16T00:00:00Z",
            "region": "eu-central-1",
            "resources": [],
            "detail": {
                "event_id": "abc123",
                "event_type": event_type,
                "version": 1,
                "tenant_id": tenant_id,
                "occurred_at": "2026-08-16T00:00:00Z",
                "payload": {"insight_id": insight_id, "tags": ["work"]},
            },
        }
    )


def _sqs_event(*bodies: str) -> dict:
    return {"Records": [{"messageId": f"m-{i}", "body": b} for i, b in enumerate(bodies)]}


def test_handle_valid_record_logs_structurally_and_skips_dlq(
    caplog: pytest.LogCaptureFixture,
) -> None:
    dlq = SpyDlq()
    handler = Handler(dlq)

    with caplog.at_level("INFO"):
        handler.handle(_sqs_event(_envelope()))

    assert dlq.sent == []
    record = next(r for r in caplog.records if r.getMessage() == "received domain event")
    assert record.tenant_id == "t1"
    assert record.insight_id == "i1"
    assert record.event_type == "InsightEnriched"


def test_handle_malformed_json_routes_to_dlq_without_raising() -> None:
    dlq = SpyDlq()
    handler = Handler(dlq)

    handler.handle(_sqs_event("{not json"))

    assert len(dlq.sent) == 1
    assert dlq.sent[0][0] == "{not json"


def test_handle_missing_envelope_field_routes_to_dlq() -> None:
    dlq = SpyDlq()
    handler = Handler(dlq)
    body = json.dumps(
        {"detail-type": "InsightEnriched", "detail": {"event_type": "InsightEnriched"}}
    )

    handler.handle(_sqs_event(body))

    assert len(dlq.sent) == 1


def test_handle_poison_record_does_not_block_the_rest_of_the_batch(
    caplog: pytest.LogCaptureFixture,
) -> None:
    dlq = SpyDlq()
    handler = Handler(dlq)

    with caplog.at_level("INFO"):
        handler.handle(_sqs_event("{not json", _envelope(insight_id="i2")))

    assert len(dlq.sent) == 1
    logged = [r for r in caplog.records if r.getMessage() == "received domain event"]
    assert len(logged) == 1
    assert logged[0].insight_id == "i2"


def test_handle_dlq_send_failure_is_swallowed_as_degraded_path() -> None:
    dlq = SpyDlq(send_error=RuntimeError("sqs unavailable"))
    handler = Handler(dlq)

    handler.handle(_sqs_event("{not json"))  # must not raise

    assert len(dlq.sent) == 1


def test_handle_unexpected_error_propagates_for_redelivery(monkeypatch: pytest.MonkeyPatch) -> None:
    dlq = SpyDlq()
    handler = Handler(dlq)
    monkeypatch.setattr(
        "ipp_ai.adapters.inbound.event_subscription._parse_envelope",
        lambda body: (_ for _ in ()).throw(RuntimeError("bug")),
    )

    with pytest.raises(RuntimeError):
        handler.handle(_sqs_event(_envelope()))

    assert dlq.sent == []
