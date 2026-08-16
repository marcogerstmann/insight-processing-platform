"""boto3-backed SecretProvider — the AWS counterpart to internal/adapters/outbound/ssm.

Satisfies ipp_ai.ports.SecretProvider structurally. It does not import
ipp_ai.ports — that's what makes the boundary a real, checkable property
(via ruff's banned-api rule on the reverse direction) rather than a
convention. See ADR-017.
"""

from __future__ import annotations

from typing import Any

import boto3


class SsmSecretProvider:
    """Fetches a decrypted SSM parameter by its full path."""

    def __init__(self, client: Any | None = None) -> None:
        self._client = client or boto3.client("ssm")

    def get(self, name: str) -> str:
        response = self._client.get_parameter(Name=name, WithDecryption=True)
        return response["Parameter"]["Value"]
