"""Cognito client_credentials token client — IPP-94.

Machine-to-machine auth is what makes "the AI service writes only through
the Go REST API, never directly to DynamoDB" (services/ai/README.md)
actually enforceable rather than a convention someone forgets under
deadline pressure. This is the Python half of that mechanism: fetching and
caching the bearer token this service's own Cognito app client
(terraform/envs/dev/rest-api.tf's aws_cognito_user_pool_client.agent) is
entitled to via the OAuth2 client_credentials grant.

Not yet used by anything — REL 4 (IPP-100) is the first caller, when the AI
service starts writing relationships through the Go API instead of around
it. This ticket only builds the mechanism that boundary depends on.
"""

from __future__ import annotations

import json
import time
import urllib.error
import urllib.parse
import urllib.request

_TIMEOUT_SECONDS = 8
_MAX_ATTEMPTS = 3

# Refresh this many seconds before the token's own expiry, so a request
# already in flight never races a token about to be rejected server-side.
_EXPIRY_SAFETY_MARGIN_SECONDS = 30


class CognitoServiceTokenClient:
    """Fetches a client_credentials access token and caches it until shortly
    before it expires, refreshing on demand rather than once per request —
    IPP-94's explicit requirement, and the reason this isn't just a
    stateless function like OpenAiEmbeddingClient.embed.
    """

    def __init__(self, token_endpoint: str, client_id: str, client_secret: str, scope: str) -> None:
        self._token_endpoint = token_endpoint
        self._client_id = client_id
        self._client_secret = client_secret
        self._scope = scope
        self._token: str | None = None
        self._expires_at: float = 0.0

    def token(self) -> str:
        if self._token is None or time.monotonic() >= self._expires_at:
            self._token, self._expires_at = self._fetch()
        return self._token

    def _fetch(self) -> tuple[str, float]:
        body = urllib.parse.urlencode(
            {
                "grant_type": "client_credentials",
                "client_id": self._client_id,
                "client_secret": self._client_secret,
                "scope": self._scope,
            }
        ).encode()
        request = urllib.request.Request(
            self._token_endpoint,
            data=body,
            method="POST",
            headers={"Content-Type": "application/x-www-form-urlencoded"},
        )
        payload = _send_json(request)
        # monotonic, not wall-clock: a system clock adjustment must never
        # make a cached token look valid for longer (or shorter) than it is.
        expires_at = time.monotonic() + payload["expires_in"] - _EXPIRY_SAFETY_MARGIN_SECONDS
        return payload["access_token"], expires_at


def _send_json(request: urllib.request.Request) -> dict:
    # _MAX_ATTEMPTS >= 1 guarantees this placeholder is always overwritten below.
    last_error: Exception = RuntimeError("unreachable")
    for _ in range(_MAX_ATTEMPTS):
        try:
            with urllib.request.urlopen(request, timeout=_TIMEOUT_SECONDS) as response:
                return json.load(response)
        except urllib.error.HTTPError as exc:
            if exc.code < 500:
                raise  # bad secret / bad request — retrying changes nothing
            last_error = exc
        except (urllib.error.URLError, TimeoutError) as exc:
            last_error = exc
    raise last_error
