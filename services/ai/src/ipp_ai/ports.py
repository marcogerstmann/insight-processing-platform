"""Ports the application depends on.

Go declares one `interface` per file under internal/ports; here every port
is a `typing.Protocol`, collected in this one module — there are few enough
of them that one file per port would be Go ceremony, not Python idiom.

`Protocol`, not `abc.ABC`: an adapter satisfies a Protocol structurally,
without importing it, which is what keeps the dependency direction
(adapters depend on nothing in this service) intact. See ADR-017.
"""

from __future__ import annotations

from typing import Protocol


class SecretProvider(Protocol):
    """Resolves a secret by name. Mirrors internal/ports.SecretProvider."""

    def get(self, name: str) -> str: ...
