"""OpenAI-backed EmbeddingClient.

Replaces the Voyage adapter (IPP-135). Voyage existed only because
Anthropic served no embeddings; now that one provider covers enrichment
and embeddings both, this service and the Go worker share a single key —
see docs/adr/018-one-provider-for-model-capabilities.md.

Satisfies ipp_ai.ports.EmbeddingClient structurally; it does not import
ipp_ai.ports (see ADR-017). Still plain `urllib.request` rather than the
`openai` SDK: one POST, one response, and the SDK would be a dependency
carried into the Lambda image for a single call.

Bounded the same way internal/adapters/outbound/openai.Client is (ADR-013's
discipline, applied to a second capability): a timeout per attempt, capped
retries, and input truncated before it's sent. The bound is on total
wall-clock time across every attempt, not per attempt, because it has to
fit inside the ai Lambda's own 30s timeout (terraform/envs/dev/ai.tf)
alongside the DynamoDB calls either side of it.
"""

from __future__ import annotations

import json
import urllib.error
import urllib.request

_ENDPOINT = "https://api.openai.com/v1/embeddings"
_MODEL = "text-embedding-3-small"

# Not the model's 1536 default. This system compares vectors by brute-force
# arithmetic with no vector index, and stores them as DynamoDB
# Decimal lists — so width sets item size, read-capacity cost and
# per-comparison cost at once. text-embedding-3-* are trained so a shortened
# vector keeps its concept-representing properties, which makes 512 a ~3x
# saving on all three for a corpus this size rather than a quality tax.
# Revisit if recall on real queries disappoints; it is a re-embed, not a
# schema change, because `dimension` travels with every stored vector.
_DIMENSION = 512

_TIMEOUT_SECONDS = 8
_MAX_ATTEMPTS = 3

# TRADE-OFF: character count as a stand-in for token count — no tokenizer
# dependency for one truncation guard. text-embedding-3-small's real limit
# is 8191 tokens, so this cuts in well before that; swap in a real tokenizer
# if truncation ever visibly clips meaningful input.
_MAX_INPUT_CHARS = 8000


class OpenAiEmbeddingClient:
    """Embeds text via OpenAI's `/v1/embeddings` endpoint."""

    model = _MODEL
    dimension = _DIMENSION

    def __init__(self, api_key: str) -> None:
        self._api_key = api_key

    def embed(self, text: str) -> tuple[float, ...]:
        body = json.dumps(
            {
                "input": [text[:_MAX_INPUT_CHARS]],
                "model": self.model,
                "dimensions": self.dimension,
            }
        ).encode()
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
