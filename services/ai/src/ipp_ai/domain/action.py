"""A drafted weekly-plan action — pure value type, no I/O.

The read side's counterpart to PLAN 4's persisted action (IPP-106, not yet
built). Its `supporting_insight_ids` are exactly what the model returned —
unverified until application/action_generation.py checks them against the
ids SelectedContext actually supplied; there is deliberately no separate
"validated" type, since validation only ever narrows this same tuple or
drops the whole action, never adds or reshapes a field.
"""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class Action:
    title: str
    why: str
    supporting_insight_ids: tuple[str, ...]
