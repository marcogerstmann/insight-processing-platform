from __future__ import annotations

import json
import urllib.error
import urllib.request

import pytest

from ipp_ai.adapters.outbound.plan_result_api import GoApiPlanResultWriter
from ipp_ai.domain.action import Action
from ipp_ai.errors import PermanentError, PlanAlreadyResolved


class _FakeResponse:
    def __init__(self, payload: dict | None = None) -> None:
        self._body = json.dumps(payload).encode() if payload is not None else b""

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


def test_set_ready_puts_actions_to_the_result_endpoint(monkeypatch: pytest.MonkeyPatch) -> None:
    seen: dict = {}

    def fake_urlopen(request: urllib.request.Request, timeout: float) -> _FakeResponse:
        seen["request"] = request
        return _FakeResponse()

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)

    writer = GoApiPlanResultWriter("https://api.example.com/", _FakeTokenClient("tok-1"))
    writer.set_ready(
        "t1",
        "p1",
        [
            Action(
                title="Ship it",
                why="Directly serves the focus.",
                supporting_insight_ids=("i0", "i1"),
            )
        ],
    )

    request = seen["request"]
    assert request.full_url == "https://api.example.com/v1/tenants/t1/weekly-plans/p1/result"
    assert request.get_method() == "PUT"
    assert request.get_header("Authorization") == "Bearer tok-1"
    assert json.loads(request.data) == {
        "status": "ready",
        "actions": [
            {
                "title": "Ship it",
                "why": "Directly serves the focus.",
                "supporting_insight_ids": ["i0", "i1"],
            }
        ],
    }


def test_set_failed_puts_a_failure_reason(monkeypatch: pytest.MonkeyPatch) -> None:
    seen: dict = {}

    def fake_urlopen(request: urllib.request.Request, timeout: float) -> _FakeResponse:
        seen["request"] = request
        return _FakeResponse()

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)

    GoApiPlanResultWriter("https://api.example.com", _FakeTokenClient()).set_failed(
        "t1", "p1", "not enough material"
    )

    assert json.loads(seen["request"].data) == {
        "status": "failed",
        "failure_reason": "not enough material",
    }


def test_set_ready_retries_on_5xx_and_succeeds(monkeypatch: pytest.MonkeyPatch) -> None:
    attempts = {"n": 0}

    def fake_urlopen(request: urllib.request.Request, timeout: float) -> _FakeResponse:
        attempts["n"] += 1
        if attempts["n"] < 2:
            raise urllib.error.HTTPError(request.full_url, 503, "unavailable", {}, None)
        return _FakeResponse()

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)

    GoApiPlanResultWriter("https://api.example.com", _FakeTokenClient()).set_ready("t1", "p1", [])

    assert attempts["n"] == 2


def test_set_ready_does_not_retry_on_4xx(monkeypatch: pytest.MonkeyPatch) -> None:
    attempts = {"n": 0}

    def fake_urlopen(request: urllib.request.Request, timeout: float) -> _FakeResponse:
        attempts["n"] += 1
        raise urllib.error.HTTPError(request.full_url, 400, "bad request", {}, None)

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)

    writer = GoApiPlanResultWriter("https://api.example.com", _FakeTokenClient())
    with pytest.raises(urllib.error.HTTPError):
        writer.set_ready("t1", "p1", [])

    assert attempts["n"] == 1


def test_set_ready_conflict_raises_plan_already_resolved_not_retried(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """PLAN 5 (IPP-107): a redelivered write that lost the conditional-write
    race gets a 409 — translated to PlanAlreadyResolved so the event handler
    can tell "already resolved" apart from a genuine failure worth
    redelivering for.
    """
    attempts = {"n": 0}

    def fake_urlopen(request: urllib.request.Request, timeout: float) -> _FakeResponse:
        attempts["n"] += 1
        raise urllib.error.HTTPError(request.full_url, 409, "conflict", {}, None)

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)

    writer = GoApiPlanResultWriter("https://api.example.com", _FakeTokenClient())
    with pytest.raises(PlanAlreadyResolved):
        writer.set_ready("t1", "p1", [])

    assert attempts["n"] == 1


def test_status_returns_the_plans_status(monkeypatch: pytest.MonkeyPatch) -> None:
    seen: dict = {}

    def fake_urlopen(request: urllib.request.Request, timeout: float) -> _FakeResponse:
        seen["request"] = request
        return _FakeResponse({"status": "ready"})

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)

    writer = GoApiPlanResultWriter("https://api.example.com", _FakeTokenClient("tok-1"))
    status = writer.status("t1", "p1")

    request = seen["request"]
    assert request.full_url == "https://api.example.com/v1/tenants/t1/weekly-plans/p1/status"
    assert request.get_method() == "GET"
    assert request.get_header("Authorization") == "Bearer tok-1"
    assert status == "ready"


def test_status_unknown_plan_raises_permanent_error(monkeypatch: pytest.MonkeyPatch) -> None:
    def fake_urlopen(request: urllib.request.Request, timeout: float) -> _FakeResponse:
        raise urllib.error.HTTPError(request.full_url, 404, "not found", {}, None)

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)

    writer = GoApiPlanResultWriter("https://api.example.com", _FakeTokenClient())
    with pytest.raises(PermanentError):
        writer.status("t1", "p-missing")
