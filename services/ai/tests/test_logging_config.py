import json
import logging
import sys

from ipp_ai.logging_config import JsonFormatter


def test_json_formatter_emits_valid_json_with_extra_fields() -> None:
    formatter = JsonFormatter()
    record = logging.LogRecord(
        name="ipp_ai.test",
        level=logging.INFO,
        pathname=__file__,
        lineno=1,
        msg="received domain event",
        args=(),
        exc_info=None,
    )
    record.tenant_id = "t1"
    record.insight_id = "i1"
    record.event_type = "InsightEnriched"

    parsed = json.loads(formatter.format(record))

    assert parsed["msg"] == "received domain event"
    assert parsed["level"] == "info"
    assert parsed["tenant_id"] == "t1"
    assert parsed["insight_id"] == "i1"
    assert parsed["event_type"] == "InsightEnriched"
    assert "time" in parsed


def test_json_formatter_includes_exception_text() -> None:
    formatter = JsonFormatter()
    try:
        raise ValueError("boom")
    except ValueError:
        record = logging.LogRecord(
            name="ipp_ai.test",
            level=logging.ERROR,
            pathname=__file__,
            lineno=1,
            msg="failed",
            args=(),
            exc_info=True,
        )
        record.exc_info = sys.exc_info()

    parsed = json.loads(formatter.format(record))

    assert "boom" in parsed["err"]
