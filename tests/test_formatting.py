from datetime import timezone
from unittest.mock import patch

import click
import pytest

from slack_cli.formatting import format_message, parse_slack_url, parse_time


# ── parse_slack_url ───────────────────────────────────────────────────────────


def test_parse_slack_url_basic():
    url = "https://myteam.slack.com/archives/C12345678/p1741234567123456"
    channel, message_ts, thread_ts = parse_slack_url(url)
    assert channel == "C12345678"
    assert message_ts == "1741234567.123456"
    assert thread_ts is None


def test_parse_slack_url_with_thread_ts():
    url = "https://myteam.slack.com/archives/C12345678/p1741234567123456?thread_ts=1741234560.000100&cid=C12345678"
    channel, message_ts, thread_ts = parse_slack_url(url)
    assert channel == "C12345678"
    assert message_ts == "1741234567.123456"
    assert thread_ts == "1741234560.000100"


def test_parse_slack_url_invalid_path():
    with pytest.raises(click.ClickException, match="Unrecognized Slack URL"):
        parse_slack_url("https://myteam.slack.com/messages/C12345678")


def test_parse_slack_url_invalid_ts_format():
    with pytest.raises(click.ClickException, match="timestamp"):
        parse_slack_url("https://myteam.slack.com/archives/C12345678/notavalidts")


# ── parse_time ────────────────────────────────────────────────────────────────


def test_parse_time_minutes():
    import time
    result = parse_time("30m")
    assert abs(result - (time.time() - 30 * 60)) < 2


def test_parse_time_hours():
    import time
    result = parse_time("3h")
    assert abs(result - (time.time() - 3 * 3600)) < 2


def test_parse_time_days():
    import time
    result = parse_time("2d")
    assert abs(result - (time.time() - 2 * 86400)) < 2


def test_parse_time_weeks():
    import time
    result = parse_time("1w")
    assert abs(result - (time.time() - 7 * 86400)) < 2


def test_parse_time_absolute_date():
    result = parse_time("2026-03-01")
    from datetime import datetime
    expected = datetime(2026, 3, 1, tzinfo=timezone.utc).timestamp()
    assert result == expected


def test_parse_time_absolute_datetime():
    result = parse_time("2026-03-01T14:00:00")
    from datetime import datetime
    expected = datetime(2026, 3, 1, 14, 0, 0, tzinfo=timezone.utc).timestamp()
    assert result == expected


def test_parse_time_unix_timestamp():
    assert parse_time("1741234567") == 1741234567.0


def test_parse_time_invalid():
    with pytest.raises(click.ClickException, match="Cannot parse time"):
        parse_time("not-a-time")


# ── format_message ────────────────────────────────────────────────────────────


def test_format_message_basic():
    msg = {"ts": "1741234567.000000", "user": "U123", "text": "hello"}
    result = format_message(msg)
    assert result["ts"] == "1741234567.000000"
    assert result["user"] == "U123"
    assert result["text"] == "hello"
    assert "time" in result


def test_format_message_truncates_long_text():
    msg = {"ts": "1741234567.000000", "text": "x" * 600}
    result = format_message(msg)
    assert len(result["text"]) == 500


def test_format_message_omits_empty_text():
    msg = {"ts": "1741234567.000000", "text": "   "}
    result = format_message(msg)
    assert "text" not in result


def test_format_message_includes_reply_count():
    msg = {"ts": "1741234567.000000", "reply_count": 5}
    result = format_message(msg)
    assert result["reply_count"] == 5


def test_format_message_omits_zero_reply_count():
    msg = {"ts": "1741234567.000000", "reply_count": 0}
    result = format_message(msg)
    assert "reply_count" not in result


def test_format_message_includes_reactions():
    msg = {"ts": "1741234567.000000", "reactions": [{"name": "thumbsup", "count": 3}]}
    result = format_message(msg)
    assert result["reactions"] == ["thumbsup(3)"]


def test_format_message_attachment():
    msg = {
        "ts": "1741234567.000000",
        "attachments": [{"title": "Alert", "text": "Something broke", "color": "danger"}],
    }
    result = format_message(msg)
    assert result["attachment"]["title"] == "Alert"
    assert result["attachment"]["color"] == "danger"


def test_format_message_missing_ts():
    result = format_message({"text": "hello"})
    assert "ts" not in result
    assert "time" not in result
