"""Secret resolution — the Python counterpart to internal/envutil.ResolveSecret.

Application-layer code reads the environment directly, same as
internal/application/tenant/resolver.go does in Go; a single lookup like
this doesn't earn a separate config-loading port.
"""

from __future__ import annotations

import os

from ipp_ai.domain.secret import parse_secret_ref
from ipp_ai.ports import SecretProvider


def resolve_secret(key: str, provider: SecretProvider | None) -> str:
    """Read the env var named `key`.

    An "ssm:"-prefixed value is resolved via `provider`. Returns "" if the
    env var is unset or empty — callers decide whether absence is an error.
    """
    raw = os.environ.get(key, "").strip()
    if not raw:
        return ""
    ref = parse_secret_ref(raw)
    if not ref.is_ssm_path:
        return ref.value
    if provider is None:
        raise RuntimeError(f"SSM provider required for {key} but not configured")
    return provider.get(ref.value)
