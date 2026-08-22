from __future__ import annotations

import json
import urllib.error
import urllib.request

import pytest

from ipp_ai.adapters.outbound.relationship_api import GoApiRelationshipWriter
from ipp_ai.domain.relationship import Relationship, RelationType


class _FakeResponse:
    def __init__(self, payload: dict) -> None:
        self._body = json.dumps(payload).encode()

    def __enter__(self) -> _FakeResponse:
        return self

    def __exit__(self, *exc: object) -> bool:
        return False

    def read(self) -> bytes:
        return self._body


class _FakeTokenClient:
    def __init__(self, token: str = "agent-token") -> None:
        self._token = token

    def token(self) -> str:
        return self._token


def _relationship() -> Relationship:
    return Relationship(
        tenant_id="t1",
        from_insight_id="i1",
        to_insight_id="i2",
        relation_type=RelationType.SUPPORTS,
        confidence=0.9,
        rationale="both describe the same idea",
    )


def test_put_posts_to_the_relationships_endpoint_with_a_bearer_token(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    seen: dict = {}

    def fake_urlopen(request: urllib.request.Request, timeout: float) -> _FakeResponse:
        seen["request"] = request
        return _FakeResponse({"from_insight_id": "i1"})

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)

    writer = GoApiRelationshipWriter("https://api.example.com/", _FakeTokenClient("tok-1"))
    writer.put(_relationship())

    request = seen["request"]
    assert request.full_url == "https://api.example.com/v1/tenants/t1/insights/i1/relationships"
    assert request.get_header("Authorization") == "Bearer tok-1"
    assert json.loads(request.data) == {
        "to_insight_id": "i2",
        "type": "supports",
        "confidence": 0.9,
        "rationale": "both describe the same idea",
    }


def test_put_retries_on_5xx_and_succeeds(monkeypatch: pytest.MonkeyPatch) -> None:
    attempts = {"n": 0}

    def fake_urlopen(request: urllib.request.Request, timeout: float) -> _FakeResponse:
        attempts["n"] += 1
        if attempts["n"] < 2:
            raise urllib.error.HTTPError(request.full_url, 503, "unavailable", {}, None)
        return _FakeResponse({"from_insight_id": "i1"})

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)

    GoApiRelationshipWriter("https://api.example.com", _FakeTokenClient()).put(_relationship())

    assert attempts["n"] == 2


def test_put_does_not_retry_on_4xx(monkeypatch: pytest.MonkeyPatch) -> None:
    attempts = {"n": 0}

    def fake_urlopen(request: urllib.request.Request, timeout: float) -> _FakeResponse:
        attempts["n"] += 1
        raise urllib.error.HTTPError(request.full_url, 400, "bad request", {}, None)

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)

    with pytest.raises(urllib.error.HTTPError):
        GoApiRelationshipWriter("https://api.example.com", _FakeTokenClient()).put(_relationship())

    assert attempts["n"] == 1
