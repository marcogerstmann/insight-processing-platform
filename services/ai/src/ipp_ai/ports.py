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

from ipp_ai.domain.insight import Insight


class SecretProvider(Protocol):
    """Resolves a secret by name. Mirrors internal/ports.SecretProvider."""

    def get(self, name: str) -> str: ...


class InsightReader(Protocol):
    """Read-only access to the insights table.

    Deliberately not internal/ports.InsightRepository translated wholesale:
    this service never writes, so there is no CreateIfAbsent/Update here —
    see services/ai/README.md for why writes go through the Go REST API
    instead. get_by_id has no Go counterpart either; the Go repository never
    needed one; this service does, to load a single insight for an agent.
    """

    def get_by_id(self, tenant_id: str, insight_id: str) -> Insight | None: ...
    def list_by_tenant(self, tenant_id: str) -> list[Insight]: ...
    def list_by_tag(self, tenant_id: str, tag: str) -> list[Insight]: ...


class DlqPublisher(Protocol):
    """Forwards a failed record's raw body to a dead-letter queue, tagged
    with the failure reason. Mirrors internal/ports.DLQPublisher — narrower,
    since nothing here needs to forward SQS message attributes (this
    service's only inbound record shape is an EventBridge envelope, which
    carries none worth preserving).
    """

    def send(self, body: str, reason: str) -> None: ...
