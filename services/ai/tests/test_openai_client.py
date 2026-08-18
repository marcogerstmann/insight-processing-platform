from __future__ import annotations

import json
import urllib.error
import urllib.request

import pytest

from ipp_ai.adapters.outbound.openai import OpenAiEmbeddingClient, OpenAiRelationLabeler
from ipp_ai.domain.relationship import RelationType


class _FakeResponse:
    def __init__(self, payload: dict) -> None:
        self._body = json.dumps(payload).encode()

    def __enter__(self) -> _FakeResponse:
        return self

    def __exit__(self, *exc: object) -> bool:
        return False

    def read(self) -> bytes:
        return self._body


def _embedding_payload(vector: list[float]) -> dict:
    return {"data": [{"embedding": vector}], "model": "text-embedding-3-small"}


def test_embed_sends_the_api_key_model_and_dimensions_and_returns_the_vector(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    seen: dict = {}

    def fake_urlopen(request: urllib.request.Request, timeout: float) -> _FakeResponse:
        seen["request"] = request
        seen["timeout"] = timeout
        return _FakeResponse(_embedding_payload([0.1, 0.2]))

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)

    client = OpenAiEmbeddingClient(api_key="secret-key")
    vector = client.embed("some highlight text")

    assert vector == (0.1, 0.2)
    assert seen["request"].get_header("Authorization") == "Bearer secret-key"
    body = json.loads(seen["request"].data)
    assert body == {
        "input": ["some highlight text"],
        "model": "text-embedding-3-small",
        # The width is a deliberate choice, not the model default — if this
        # stops being sent, stored vectors silently change shape.
        "dimensions": 512,
    }


def test_embed_truncates_long_input(monkeypatch: pytest.MonkeyPatch) -> None:
    seen: dict = {}

    def fake_urlopen(request: urllib.request.Request, timeout: float) -> _FakeResponse:
        seen["body"] = json.loads(request.data)
        return _FakeResponse(_embedding_payload([0.0]))

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)

    client = OpenAiEmbeddingClient(api_key="k")
    client.embed("x" * 20_000)

    assert len(seen["body"]["input"][0]) == 8000


def test_embed_retries_on_5xx_then_succeeds(monkeypatch: pytest.MonkeyPatch) -> None:
    attempts = {"n": 0}

    def fake_urlopen(request: urllib.request.Request, timeout: float) -> _FakeResponse:
        attempts["n"] += 1
        if attempts["n"] < 2:
            raise urllib.error.HTTPError("url", 503, "unavailable", None, None)
        return _FakeResponse(_embedding_payload([0.4]))

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)

    client = OpenAiEmbeddingClient(api_key="k")
    assert client.embed("text") == (0.4,)
    assert attempts["n"] == 2


def test_embed_does_not_retry_on_4xx(monkeypatch: pytest.MonkeyPatch) -> None:
    attempts = {"n": 0}

    def fake_urlopen(request: urllib.request.Request, timeout: float) -> _FakeResponse:
        attempts["n"] += 1
        raise urllib.error.HTTPError("url", 401, "unauthorized", None, None)

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)

    client = OpenAiEmbeddingClient(api_key="bad-key")
    with pytest.raises(urllib.error.HTTPError):
        client.embed("text")
    assert attempts["n"] == 1


def test_embed_raises_after_exhausting_retries_on_repeated_5xx(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    attempts = {"n": 0}

    def fake_urlopen(request: urllib.request.Request, timeout: float) -> _FakeResponse:
        attempts["n"] += 1
        raise urllib.error.HTTPError("url", 500, "error", None, None)

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)

    client = OpenAiEmbeddingClient(api_key="k")
    with pytest.raises(urllib.error.HTTPError):
        client.embed("text")
    assert attempts["n"] == 3


def _relation_payload(
    *,
    relation_type: str = "supports",
    confidence: float = 0.9,
    rationale: str = "Both make the same claim about delayed gratification.",
    model: str = "gpt-5.6-luna",
) -> dict:
    content = json.dumps(
        {"relation_type": relation_type, "confidence": confidence, "rationale": rationale}
    )
    return {
        "model": model,
        "choices": [{"message": {"content": content}}],
        "usage": {"prompt_tokens": 42, "completion_tokens": 17},
    }


def test_label_sends_both_highlights_and_a_forced_json_schema(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    seen: dict = {}

    def fake_urlopen(request: urllib.request.Request, timeout: float) -> _FakeResponse:
        seen["request"] = request
        return _FakeResponse(_relation_payload())

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)

    labeler = OpenAiRelationLabeler(api_key="secret-key")
    judgement = labeler.label("highlight A text", "highlight B text")

    assert judgement.relation_type is RelationType.SUPPORTS
    assert judgement.confidence == 0.9
    assert judgement.rationale == "Both make the same claim about delayed gratification."

    body = json.loads(seen["request"].data)
    assert "highlight A text" in body["messages"][1]["content"]
    assert "highlight B text" in body["messages"][1]["content"]
    schema = body["response_format"]["json_schema"]
    assert schema["strict"] is True
    assert schema["schema"]["properties"]["relation_type"]["enum"] == [
        "supports",
        "contradicts",
        "extends",
        "example_of",
        "same_topic",
    ]


def test_label_rejects_a_relation_type_outside_the_enum(monkeypatch: pytest.MonkeyPatch) -> None:
    def fake_urlopen(request: urllib.request.Request, timeout: float) -> _FakeResponse:
        return _FakeResponse(_relation_payload(relation_type="loosely_similar"))

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)

    labeler = OpenAiRelationLabeler(api_key="k")
    with pytest.raises(ValueError):
        labeler.label("a", "b")


def test_label_retries_on_5xx_then_succeeds(monkeypatch: pytest.MonkeyPatch) -> None:
    attempts = {"n": 0}

    def fake_urlopen(request: urllib.request.Request, timeout: float) -> _FakeResponse:
        attempts["n"] += 1
        if attempts["n"] < 2:
            raise urllib.error.HTTPError("url", 503, "unavailable", None, None)
        return _FakeResponse(_relation_payload())

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)

    labeler = OpenAiRelationLabeler(api_key="k")
    labeler.label("a", "b")
    assert attempts["n"] == 2
