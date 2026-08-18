from __future__ import annotations

import json
import time
import urllib.error
import urllib.parse
import urllib.request

import pytest

from ipp_ai.adapters.outbound.cognito import CognitoServiceTokenClient


class _FakeResponse:
    def __init__(self, payload: dict) -> None:
        self._body = json.dumps(payload).encode()

    def __enter__(self) -> _FakeResponse:
        return self

    def __exit__(self, *exc: object) -> bool:
        return False

    def read(self) -> bytes:
        return self._body


def _token_payload(*, access_token: str = "token-1", expires_in: int = 3600) -> dict:
    return {"access_token": access_token, "expires_in": expires_in, "token_type": "Bearer"}


def _client() -> CognitoServiceTokenClient:
    return CognitoServiceTokenClient(
        token_endpoint="https://ipp-dev-agent-auth.auth.eu-central-1.amazoncognito.com/oauth2/token",
        client_id="agent-client-id",
        client_secret="agent-client-secret",
        scope="ipp/agent.write",
    )


def test_token_sends_client_credentials_grant(monkeypatch: pytest.MonkeyPatch) -> None:
    seen: dict = {}

    def fake_urlopen(request: urllib.request.Request, timeout: float) -> _FakeResponse:
        seen["request"] = request
        return _FakeResponse(_token_payload())

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)

    client = _client()
    assert client.token() == "token-1"

    body = urllib.parse.parse_qs(seen["request"].data.decode())
    assert body == {
        "grant_type": ["client_credentials"],
        "client_id": ["agent-client-id"],
        "client_secret": ["agent-client-secret"],
        "scope": ["ipp/agent.write"],
    }
    assert seen["request"].get_header("Content-type") == "application/x-www-form-urlencoded"


def test_token_is_cached_across_calls(monkeypatch: pytest.MonkeyPatch) -> None:
    attempts = {"n": 0}

    def fake_urlopen(request: urllib.request.Request, timeout: float) -> _FakeResponse:
        attempts["n"] += 1
        return _FakeResponse(_token_payload())

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)

    client = _client()
    assert client.token() == "token-1"
    assert client.token() == "token-1"
    assert attempts["n"] == 1


def test_token_is_refreshed_after_expiry(monkeypatch: pytest.MonkeyPatch) -> None:
    responses = iter(
        [
            _token_payload(access_token="token-1", expires_in=60),
            _token_payload(access_token="token-2"),
        ]
    )

    def fake_urlopen(request: urllib.request.Request, timeout: float) -> _FakeResponse:
        return _FakeResponse(next(responses))

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)

    fake_now = {"t": 1000.0}
    monkeypatch.setattr(time, "monotonic", lambda: fake_now["t"])

    client = _client()
    assert client.token() == "token-1"

    # Still within the 30s safety margin of the 60s expiry — no refetch yet.
    fake_now["t"] += 20
    assert client.token() == "token-1"

    # Past the safety margin now.
    fake_now["t"] += 20
    assert client.token() == "token-2"


def test_token_retries_on_5xx_then_succeeds(monkeypatch: pytest.MonkeyPatch) -> None:
    attempts = {"n": 0}

    def fake_urlopen(request: urllib.request.Request, timeout: float) -> _FakeResponse:
        attempts["n"] += 1
        if attempts["n"] < 2:
            raise urllib.error.HTTPError("url", 503, "unavailable", None, None)
        return _FakeResponse(_token_payload())

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)

    client = _client()
    assert client.token() == "token-1"
    assert attempts["n"] == 2


def test_token_does_not_retry_on_4xx(monkeypatch: pytest.MonkeyPatch) -> None:
    attempts = {"n": 0}

    def fake_urlopen(request: urllib.request.Request, timeout: float) -> _FakeResponse:
        attempts["n"] += 1
        raise urllib.error.HTTPError("url", 400, "invalid_client", None, None)

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)

    client = _client()
    with pytest.raises(urllib.error.HTTPError):
        client.token()
    assert attempts["n"] == 1
