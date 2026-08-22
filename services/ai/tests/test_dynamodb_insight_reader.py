from datetime import datetime

import pytest
from botocore.stub import ANY

from ipp_ai.domain.insight import Enrichment, Insight
from ipp_ai.domain.relationship import RelatedInsight, RelationType
from ipp_ai.errors import PermanentError

_HIGHLIGHTED_AT = "2026-01-01T00:00:00Z"  # Go's time.RFC3339Nano for UTC


def _relationship_item(
    *,
    from_insight_id: str = "i1",
    to_insight_id: str = "i2",
    related_insight_text: str = "the other insight's text",
    relation_type: str = "supports",
    confidence: str = "0.9",
    rationale: str = "because reasons",
) -> dict:
    return {
        "pk": {"S": "TENANT#t1"},
        "sk": {"S": f"REL#{from_insight_id}#{to_insight_id}"},
        "tenant_id": {"S": "t1"},
        "from_insight_id": {"S": from_insight_id},
        "to_insight_id": {"S": to_insight_id},
        "related_insight_text": {"S": related_insight_text},
        "type": {"S": relation_type},
        "confidence": {"N": confidence},
        "rationale": {"S": rationale},
    }


def _insight_item(insight_id: str = "i1", tags: list[str] | None = None) -> dict:
    item = {
        "pk": {"S": "TENANT#t1"},
        "sk": {"S": f"INSIGHT#{insight_id}"},
        "id": {"S": insight_id},
        "tenant_id": {"S": "t1"},
        "source": {"S": "readwise"},
        "text": {"S": "hello"},
        "notes": {"S": ""},
        "highlighted_at": {"S": _HIGHLIGHTED_AT},
    }
    if tags is not None:
        item["enrichment"] = {"M": {"tags": {"L": [{"S": t} for t in tags]}}}
    return item


def test_get_by_id_found(stubbed_reader) -> None:
    stubbed_reader.stubber.add_response(
        "get_item",
        {"Item": _insight_item(tags=["work", "delegation"])},
        {"TableName": "test-insights", "Key": {"pk": "TENANT#t1", "sk": "INSIGHT#i1"}},
    )

    insight = stubbed_reader.reader.get_by_id("t1", "i1")

    assert insight == Insight(
        id="i1",
        tenant_id="t1",
        source="readwise",
        text="hello",
        notes="",
        highlighted_at=datetime.fromisoformat(_HIGHLIGHTED_AT),
        enrichment=Enrichment(tags=("work", "delegation")),
    )


def test_get_by_id_missing_is_none_not_an_exception(stubbed_reader) -> None:
    stubbed_reader.stubber.add_response(
        "get_item",
        {},
        {"TableName": "test-insights", "Key": {"pk": "TENANT#t1", "sk": "INSIGHT#missing"}},
    )

    assert stubbed_reader.reader.get_by_id("t1", "missing") is None


def test_get_by_id_malformed_item_raises_permanent_error(stubbed_reader) -> None:
    broken = _insight_item()
    del broken["text"]
    stubbed_reader.stubber.add_response(
        "get_item",
        {"Item": broken},
        {"TableName": "test-insights", "Key": {"pk": "TENANT#t1", "sk": "INSIGHT#i1"}},
    )

    with pytest.raises(PermanentError):
        stubbed_reader.reader.get_by_id("t1", "i1")


def test_list_by_tenant_returns_unenriched_and_enriched(stubbed_reader) -> None:
    stubbed_reader.stubber.add_response(
        "query",
        {"Items": [_insight_item("i1"), _insight_item("i2", tags=["work"])]},
        {
            "TableName": "test-insights",
            "KeyConditionExpression": ANY,
        },
    )

    insights = stubbed_reader.reader.list_by_tenant("t1")

    assert [i.id for i in insights] == ["i1", "i2"]
    assert insights[0].enrichment is None
    assert insights[1].enrichment == Enrichment(tags=("work",))


def test_list_by_tag_fetches_each_member_via_the_gsi(stubbed_reader) -> None:
    stubbed_reader.stubber.add_response(
        "query",
        {"Items": [{"insight_id": {"S": "i1"}}, {"insight_id": {"S": "i2"}}]},
        {
            "TableName": "test-insights",
            "IndexName": "gsi1",
            "KeyConditionExpression": ANY,
        },
    )
    stubbed_reader.stubber.add_response(
        "get_item",
        {"Item": _insight_item("i1", tags=["work"])},
        {"TableName": "test-insights", "Key": {"pk": "TENANT#t1", "sk": "INSIGHT#i1"}},
    )
    stubbed_reader.stubber.add_response(
        "get_item",
        {"Item": _insight_item("i2", tags=["work"])},
        {"TableName": "test-insights", "Key": {"pk": "TENANT#t1", "sk": "INSIGHT#i2"}},
    )

    insights = stubbed_reader.reader.list_by_tag("t1", "work")

    assert [i.id for i in insights] == ["i1", "i2"]


def test_list_by_tag_skips_orphaned_membership(stubbed_reader) -> None:
    stubbed_reader.stubber.add_response(
        "query",
        {"Items": [{"insight_id": {"S": "deleted-insight"}}]},
        {
            "TableName": "test-insights",
            "IndexName": "gsi1",
            "KeyConditionExpression": ANY,
        },
    )
    stubbed_reader.stubber.add_response(
        "get_item",
        {},
        {"TableName": "test-insights", "Key": {"pk": "TENANT#t1", "sk": "INSIGHT#deleted-insight"}},
    )

    assert stubbed_reader.reader.list_by_tag("t1", "work") == []


def test_list_by_insight_returns_the_neighbor_regardless_of_edge_direction(stubbed_reader) -> None:
    """Mirrors ListByInsightID (REL 6): a copy of the edge is filed under
    both insights, keeping the original from/to direction — the reader must
    resolve "the other side" whichever direction this particular copy was
    queried from.
    """
    stubbed_reader.stubber.add_response(
        "query",
        {
            "Items": [
                _relationship_item(from_insight_id="i2", to_insight_id="i1", confidence="0.5"),
                _relationship_item(from_insight_id="i1", to_insight_id="i3", confidence="0.9"),
            ]
        },
        {"TableName": "test-insights", "KeyConditionExpression": ANY},
    )

    related = stubbed_reader.reader.list_by_insight("t1", "i1")

    # Sorted by confidence descending, regardless of query order.
    assert [r.insight_id for r in related] == ["i3", "i2"]
    assert related[0] == RelatedInsight(
        insight_id="i3",
        text="the other insight's text",
        relation_type=RelationType.SUPPORTS,
        confidence=0.9,
        rationale="because reasons",
    )


def test_list_by_insight_no_relationships_returns_empty_not_none(stubbed_reader) -> None:
    stubbed_reader.stubber.add_response(
        "query",
        {"Items": []},
        {"TableName": "test-insights", "KeyConditionExpression": ANY},
    )

    assert stubbed_reader.reader.list_by_insight("t1", "i1") == []


def test_list_by_insight_malformed_item_raises_permanent_error(stubbed_reader) -> None:
    broken = _relationship_item()
    del broken["confidence"]
    stubbed_reader.stubber.add_response(
        "query",
        {"Items": [broken]},
        {"TableName": "test-insights", "KeyConditionExpression": ANY},
    )

    with pytest.raises(PermanentError):
        stubbed_reader.reader.list_by_insight("t1", "i1")
