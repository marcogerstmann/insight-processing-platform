"""Exception taxonomy — mirrors internal/apperr's PermanentError semantics.

There is no `(result, error)` return pair here: functions either return a
value or raise. `PermanentError` is the one exception type callers must
recognise explicitly — an inbound handler (AI 4) catches it, routes the
message to a DLQ, and continues; every other exception is transient and is
left to propagate so the runtime redelivers.
"""

from __future__ import annotations


class PermanentError(Exception):
    """A failure that will never succeed on retry — route to the DLQ.

    Wrap the underlying cause: `raise PermanentError("malformed item") from exc`.
    """
