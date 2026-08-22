"""HTTP client for REL 4's Go endpoint —
POST /v1/tenants/:tenantID/insights/:insightID/relationships.

Satisfies ipp_ai.ports.RelationshipWriter structurally (ADR-017). The only
outbound write this service makes that leaves its own AWS account: every
other write (embeddings) stays inside this service's own table; a
Relationship is domain data, so it goes through the Go API instead of
DynamoDB directly, per the Epic 3 boundary rule (services/ai/README.md).

Auth is CognitoServiceTokenClient's client_credentials token (IPP-94); the
endpoint requires the agent.write scope (router.go's auth.RequireScope).
Idempotent server-side (IPP-100: re-posting the same edge updates it), so
no idempotency handling is needed here beyond the retry-on-5xx below.
"""

from __future__ import annotations

import json
import urllib.error
import urllib.request
from typing import Protocol

from ipp_ai.domain.relationship import Relationship

_TIMEOUT_SECONDS = 8
_MAX_ATTEMPTS = 3


class _TokenSource(Protocol):
    """Structural, not imported: CognitoServiceTokenClient satisfies this
    without this adapter importing another adapter (ADR-017's TID251 rule
    applies to adapters too, not just domain/ports/application) — the
    composition root (event_subscription.py) is the only place that
    constructs and connects the two.
    """

    def token(self) -> str: ...


class GoApiRelationshipWriter:
    def __init__(self, base_url: str, token_client: _TokenSource) -> None:
        self._base_url = base_url.rstrip("/")
        self._token_client = token_client

    def put(self, relationship: Relationship) -> None:
        url = (
            f"{self._base_url}/v1/tenants/{relationship.tenant_id}"
            f"/insights/{relationship.from_insight_id}/relationships"
        )
        body = json.dumps(
            {
                "to_insight_id": relationship.to_insight_id,
                "type": relationship.relation_type.value,
                "confidence": relationship.confidence,
                "rationale": relationship.rationale,
            }
        ).encode()
        request = urllib.request.Request(
            url,
            data=body,
            method="POST",
            headers={
                "Authorization": f"Bearer {self._token_client.token()}",
                "Content-Type": "application/json",
            },
        )
        _send_json(request)


def _send_json(request: urllib.request.Request) -> dict:
    # _MAX_ATTEMPTS >= 1 guarantees this placeholder is always overwritten below.
    last_error: Exception = RuntimeError("unreachable")
    for _ in range(_MAX_ATTEMPTS):
        try:
            with urllib.request.urlopen(request, timeout=_TIMEOUT_SECONDS) as response:
                return json.load(response)
        except urllib.error.HTTPError as exc:
            if exc.code < 500:
                raise  # validation/auth failure — retrying changes nothing
            last_error = exc
        except (urllib.error.URLError, TimeoutError) as exc:
            last_error = exc
    raise last_error
