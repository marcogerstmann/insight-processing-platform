"""Secret reference parsing — pure, no I/O.

Mirrors the convention read by internal/envutil.ResolveSecret: a single env
var holds either the secret value directly, or an "ssm:"-prefixed SSM
parameter path.
"""

from __future__ import annotations

from dataclasses import dataclass

_SSM_PREFIX = "ssm:"


@dataclass(frozen=True)
class SecretRef:
    """A parsed env var value: either a literal secret or an SSM path."""

    value: str
    is_ssm_path: bool


def parse_secret_ref(raw: str) -> SecretRef:
    """Parse a raw env var value into a SecretRef.

    "ssm:/path/to/param" -> SecretRef(value="/path/to/param", is_ssm_path=True)
    "sk-ant-..."          -> SecretRef(value="sk-ant-...", is_ssm_path=False)
    """
    if raw.startswith(_SSM_PREFIX):
        return SecretRef(value=raw[len(_SSM_PREFIX) :], is_ssm_path=True)
    return SecretRef(value=raw, is_ssm_path=False)
