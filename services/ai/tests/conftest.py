"""Shared fixtures. `stubbed_reader` gives a DynamoDbInsightReader backed by
botocore's built-in Stubber — no AWS, no new dependency (botocore already
ships with boto3).
"""

from __future__ import annotations

from collections.abc import Iterator
from dataclasses import dataclass
from typing import Any

import boto3
import pytest
from botocore.stub import Stubber

from ipp_ai.adapters.outbound.dynamodb import DynamoDbInsightReader
from ipp_ai.adapters.outbound.embedding_store import DynamoDbEmbeddingWriter

_TABLE_NAME = "test-insights"
_EMBEDDINGS_TABLE_NAME = "test-embeddings"


def _stubbed_resource() -> tuple[Any, Stubber]:
    resource = boto3.resource(
        "dynamodb",
        region_name="eu-central-1",
        aws_access_key_id="testing",
        aws_secret_access_key="testing",
    )
    stubber = Stubber(resource.meta.client)
    return resource, stubber


@dataclass
class StubbedReader:
    reader: DynamoDbInsightReader
    stubber: Stubber


@pytest.fixture
def stubbed_reader() -> Iterator[StubbedReader]:
    resource, stubber = _stubbed_resource()
    stubber.activate()
    try:
        yield StubbedReader(
            reader=DynamoDbInsightReader(_TABLE_NAME, resource=resource), stubber=stubber
        )
        stubber.assert_no_pending_responses()
    finally:
        stubber.deactivate()


@dataclass
class StubbedWriter:
    writer: DynamoDbEmbeddingWriter
    stubber: Stubber


@pytest.fixture
def stubbed_writer() -> Iterator[StubbedWriter]:
    resource, stubber = _stubbed_resource()
    stubber.activate()
    try:
        yield StubbedWriter(
            writer=DynamoDbEmbeddingWriter(_EMBEDDINGS_TABLE_NAME, resource=resource),
            stubber=stubber,
        )
        stubber.assert_no_pending_responses()
    finally:
        stubber.deactivate()
