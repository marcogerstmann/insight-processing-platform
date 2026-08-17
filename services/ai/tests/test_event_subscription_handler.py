from __future__ import annotations

import json
from dataclasses import dataclass, field

import pytest

from ipp_ai.adapters.inbound.event_subscription import Handler
from ipp_ai.domain.insight import Insight


@dataclass
class SpyDlq:
    sent: list[tuple[str, str]] = field(default_factory=list)
    send_error: Exception | None = None

    def send(self, body: str, reason: str) -> None:
        self.sent.append((body, reason))
        if self.send_error is not None:
            raise self.send_error


@dataclass
class FakeReader:
    insights: dict[tuple[str, str], Insight] = field(default_factory=dict)

    def get_by_id(self, tenant_id: str, insight_id: str) -> Insight | None:
        return self.insights.get((tenant_id, insight_id))


@dataclass
class FakeEmbedder:
    model: str = "voyage-3"
    dimension: int = 3
    vector: tuple[float, ...] = (0.1, 0.2, 0.3)
    error: Exception | None = None

    def embed(self, text: str) -> tuple[float, ...]:
        if self.error is not None:
            raise self.error
        return self.vector


@dataclass
class SpyWriter:
    puts: list[object] = field(default_factory=list)

    def put(self, embedding: object) -> None:
        self.puts.append(embedding)


def _insight(insight_id: str = "i1", tenant_id: str = "t1") -> Insight:
    from datetime import datetime

    return Insight(
        id=insight_id,
        tenant_id=tenant_id,
        source="readwise",
        text="hello world",
        notes="",
        highlighted_at=datetime.fromisoformat("2026-01-01T00:00:00"),
    )


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


def _handler(
    *,
    dlq: SpyDlq | None = None,
    reader: FakeReader | None = None,
    embedder: FakeEmbedder | None = None,
    writer: SpyWriter | None = None,
) -> tuple[Handler, SpyDlq, FakeReader, FakeEmbedder, SpyWriter]:
    dlq = dlq or SpyDlq()
    reader = reader or FakeReader()
    embedder = embedder or FakeEmbedder()
    writer = writer or SpyWriter()
    return Handler(dlq, reader, embedder, writer), dlq, reader, embedder, writer


def test_handle_valid_record_logs_embeds_and_skips_dlq(caplog: pytest.LogCaptureFixture) -> None:
    reader = FakeReader({("t1", "i1"): _insight()})
    embedder = FakeEmbedder(vector=(0.1, 0.2, 0.3))
    writer = SpyWriter()
    handler, dlq, *_ = _handler(reader=reader, embedder=embedder, writer=writer)

    with caplog.at_level("INFO"):
        handler.handle(_sqs_event(_envelope()))

    assert dlq.sent == []
    record = next(r for r in caplog.records if r.getMessage() == "received domain event")
    assert record.tenant_id == "t1"
    assert record.insight_id == "i1"
    assert record.event_type == "InsightEnriched"

    assert len(writer.puts) == 1
    stored = writer.puts[0]
    assert stored.insight_id == "i1"
    assert stored.tenant_id == "t1"
    assert stored.model == "voyage-3"
    assert stored.dimension == 3
    assert stored.vector == (0.1, 0.2, 0.3)


def test_handle_malformed_json_routes_to_dlq_without_raising() -> None:
    handler, dlq, *_ = _handler()

    handler.handle(_sqs_event("{not json"))

    assert len(dlq.sent) == 1
    assert dlq.sent[0][0] == "{not json"


def test_handle_missing_envelope_field_routes_to_dlq() -> None:
    handler, dlq, *_ = _handler()
    body = json.dumps(
        {"detail-type": "InsightEnriched", "detail": {"event_type": "InsightEnriched"}}
    )

    handler.handle(_sqs_event(body))

    assert len(dlq.sent) == 1


def test_handle_unknown_insight_routes_to_dlq() -> None:
    handler, dlq, *_ = _handler(reader=FakeReader())  # empty — no insight i1

    handler.handle(_sqs_event(_envelope()))

    assert len(dlq.sent) == 1
    assert "i1" in dlq.sent[0][1]


def test_handle_poison_record_does_not_block_the_rest_of_the_batch(
    caplog: pytest.LogCaptureFixture,
) -> None:
    reader = FakeReader({("t1", "i2"): _insight("i2")})
    writer = SpyWriter()
    handler, dlq, *_ = _handler(reader=reader, writer=writer)

    with caplog.at_level("INFO"):
        handler.handle(_sqs_event("{not json", _envelope(insight_id="i2")))

    assert len(dlq.sent) == 1
    logged = [r for r in caplog.records if r.getMessage() == "received domain event"]
    assert len(logged) == 1
    assert logged[0].insight_id == "i2"
    assert len(writer.puts) == 1


def test_handle_dlq_send_failure_is_swallowed_as_degraded_path() -> None:
    dlq = SpyDlq(send_error=RuntimeError("sqs unavailable"))
    handler, *_ = _handler(dlq=dlq)

    handler.handle(_sqs_event("{not json"))  # must not raise

    assert len(dlq.sent) == 1


def test_handle_unexpected_error_propagates_for_redelivery(monkeypatch: pytest.MonkeyPatch) -> None:
    handler, dlq, *_ = _handler()
    monkeypatch.setattr(
        "ipp_ai.adapters.inbound.event_subscription._parse_envelope",
        lambda body: (_ for _ in ()).throw(RuntimeError("bug")),
    )

    with pytest.raises(RuntimeError):
        handler.handle(_sqs_event(_envelope()))

    assert dlq.sent == []


def test_handle_embedder_failure_propagates_for_redelivery() -> None:
    reader = FakeReader({("t1", "i1"): _insight()})
    embedder = FakeEmbedder(error=RuntimeError("voyage unavailable"))
    handler, dlq, *_ = _handler(reader=reader, embedder=embedder)

    with pytest.raises(RuntimeError):
        handler.handle(_sqs_event(_envelope()))

    assert dlq.sent == []  # transient — left to propagate, not DLQ'd here
