"""Voyage AI-backed EmbeddingClient — the AI service's first non-AWS
outbound adapter.

Satisfies ipp_ai.ports.EmbeddingClient structurally; it does not import
ipp_ai.ports (see ADR-017). Anthropic does not serve embeddings (IPP-97's
implementation note); Voyage is Anthropic's documented embeddings partner.
Plain `urllib.request`, not a new SDK dependency — one POST, one response,
not worth a client library.

Bounded the same way internal/adapters/outbound/anthropic.Client is
(ADR-013's discipline, applied to a second provider): a timeout per attempt,
capped retries, and input truncated before it's sent. The bound is on total
wall-clock time across every attempt, not per attempt, because it has to
fit inside the ai Lambda's own 30s timeout (terraform/envs/dev/ai.tf)
alongside the DynamoDB calls either side of it.
"""

from __future__ import annotations

import json
import urllib.error
import urllib.request

_ENDPOINT = "https://api.voyageai.com/v1/embeddings"
_MODEL = "voyage-3"
_DIMENSION = 1024  # voyage-3's fixed output size

_TIMEOUT_SECONDS = 8
_MAX_ATTEMPTS = 3

# ponytail: character count as a stand-in for token count — no tokenizer
# dependency for one truncation guard. voyage-3's real limit is ~32k tokens,
# so this cuts in well before that; swap in a real tokenizer if truncation
# ever visibly clips meaningful input.
_MAX_INPUT_CHARS = 8000


class VoyageEmbeddingClient:
    """Embeds text via Voyage AI's `/v1/embeddings` endpoint."""

    model = _MODEL
    dimension = _DIMENSION

    def __init__(self, api_key: str) -> None:
        self._api_key = api_key

    def embed(self, text: str) -> tuple[float, ...]:
        body = json.dumps({"input": [text[:_MAX_INPUT_CHARS]], "model": self.model}).encode()
        request = urllib.request.Request(
            _ENDPOINT,
            data=body,
            method="POST",
            headers={
                "Authorization": f"Bearer {self._api_key}",
                "Content-Type": "application/json",
            },
        )
        return tuple(_send(request))


def _send(request: urllib.request.Request) -> list[float]:
    # _MAX_ATTEMPTS >= 1 guarantees this placeholder is always overwritten below.
    last_error: Exception = RuntimeError("unreachable")
    for _ in range(_MAX_ATTEMPTS):
        try:
            with urllib.request.urlopen(request, timeout=_TIMEOUT_SECONDS) as response:
                payload = json.load(response)
            return payload["data"][0]["embedding"]
        except urllib.error.HTTPError as exc:
            if exc.code < 500:
                raise  # bad key / bad request — retrying changes nothing
            last_error = exc
        except (urllib.error.URLError, TimeoutError) as exc:
            last_error = exc
    raise last_error
