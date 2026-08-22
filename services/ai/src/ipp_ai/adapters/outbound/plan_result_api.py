"""HTTP client for PLAN 4's Go endpoint —
PUT /v1/tenants/:tenantID/weekly-plans/:planID/result.

Satisfies ipp_ai.ports.PlanResultWriter structurally (ADR-017). Same
boundary rule as relationship_api.py's GoApiRelationshipWriter (a WeeklyPlan
is domain data, so its result goes through the Go API rather than DynamoDB
directly) and the same auth (CognitoServiceTokenClient's agent.write token).

Not idempotent server-side the way relationships.put is: SetResult's
underlying write is conditional on the plan still being pending
(handler.go's SubmitResult doc comment), so a redelivered call after the
first one already landed gets a 409 rather than silently overwriting — that
conflict is PLAN 5/IPP-107's to handle, not retried here.
"""

from __future__ import annotations

import json
import urllib.error
import urllib.request
from typing import Protocol

from ipp_ai.domain.action import Action

_TIMEOUT_SECONDS = 8
_MAX_ATTEMPTS = 3


class _TokenSource(Protocol):
    def token(self) -> str: ...


class GoApiPlanResultWriter:
    def __init__(self, base_url: str, token_client: _TokenSource) -> None:
        self._base_url = base_url.rstrip("/")
        self._token_client = token_client

    def set_ready(self, tenant_id: str, plan_id: str, actions: list[Action]) -> None:
        self._put(
            tenant_id,
            plan_id,
            {
                "status": "ready",
                "actions": [
                    {
                        "title": action.title,
                        "why": action.why,
                        "supporting_insight_ids": list(action.supporting_insight_ids),
                    }
                    for action in actions
                ],
            },
        )

    def set_failed(self, tenant_id: str, plan_id: str, reason: str) -> None:
        self._put(tenant_id, plan_id, {"status": "failed", "failure_reason": reason})

    def _put(self, tenant_id: str, plan_id: str, body: dict) -> None:
        url = f"{self._base_url}/v1/tenants/{tenant_id}/weekly-plans/{plan_id}/result"
        request = urllib.request.Request(
            url,
            data=json.dumps(body).encode(),
            method="PUT",
            headers={
                "Authorization": f"Bearer {self._token_client.token()}",
                "Content-Type": "application/json",
            },
        )
        _send(request)


def _send(request: urllib.request.Request) -> None:
    # SubmitResult returns 204 with no body — unlike relationship_api.py's
    # _send_json, there is nothing to parse.
    last_error: Exception = RuntimeError("unreachable")
    for _ in range(_MAX_ATTEMPTS):
        try:
            with urllib.request.urlopen(request, timeout=_TIMEOUT_SECONDS):
                return
        except urllib.error.HTTPError as exc:
            if exc.code < 500:
                raise  # validation/conflict/auth failure — retrying changes nothing
            last_error = exc
        except (urllib.error.URLError, TimeoutError) as exc:
            last_error = exc
    raise last_error
