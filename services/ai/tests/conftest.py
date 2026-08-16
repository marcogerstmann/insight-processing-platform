"""Shared fixtures. `stubbed_reader` gives a DynamoDbInsightReader backed by
botocore's built-in Stubber — no AWS, no new dependency (botocore already
ships with boto3).
"""

from __future__ import annotations

from collections.abc import Iterator
from dataclasses import dataclass

import boto3
import pytest
from botocore.stub import Stubber

from ipp_ai.adapters.outbound.dynamodb import DynamoDbInsightReader

_TABLE_NAME = "test-insights"


@dataclass
class StubbedReader:
    reader: DynamoDbInsightReader
    stubber: Stubber


@pytest.fixture
def stubbed_reader() -> Iterator[StubbedReader]:
    resource = boto3.resource(
        "dynamodb",
        region_name="eu-central-1",
        aws_access_key_id="testing",
        aws_secret_access_key="testing",
    )
    table = resource.Table(_TABLE_NAME)
    stubber = Stubber(table.meta.client)
    stubber.activate()
    try:
        yield StubbedReader(
            reader=DynamoDbInsightReader(_TABLE_NAME, resource=resource), stubber=stubber
        )
        stubber.assert_no_pending_responses()
    finally:
        stubber.deactivate()
