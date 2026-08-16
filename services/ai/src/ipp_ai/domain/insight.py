"""Insight and Enrichment — pure value types, no I/O.

Mirrors internal/domain/insight.go and enrichment.go. Kept in one module,
not two: each is a handful of fields with a single consumer of the other,
and a package for two tiny types is exactly the ceremony ADR-017 argues
against.
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime


@dataclass(frozen=True)
class Enrichment:
    tags: tuple[str, ...] = ()


@dataclass(frozen=True)
class Insight:
    id: str
    tenant_id: str
    source: str
    text: str
    notes: str
    highlighted_at: datetime
    enrichment: Enrichment | None = None
