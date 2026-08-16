"""boto3-backed DlqPublisher — the read side's failure-routing counterpart to
internal/adapters/outbound/sqs.SQSDLQPublisher.

Satisfies ipp_ai.ports.DlqPublisher structurally; it does not import
ipp_ai.ports (see ADR-017).
"""

from __future__ import annotations

from typing import Any

import boto3


class SqsDlqPublisher:
    """Sends a failed record's raw body to a fixed SQS queue, tagged with why."""

    def __init__(self, queue_url: str, client: Any | None = None) -> None:
        self._queue_url = queue_url
        self._client = client or boto3.client("sqs")

    def send(self, body: str, reason: str) -> None:
        self._client.send_message(
            QueueUrl=self._queue_url,
            MessageBody=body,
            MessageAttributes={"failure_reason": {"DataType": "String", "StringValue": reason}},
        )
