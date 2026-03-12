import json
import re
import sys
from datetime import datetime, timedelta, timezone
from urllib.parse import parse_qs, urlparse

import click


def parse_slack_url(url: str) -> tuple[str, str, str | None]:
    """
    Parse a Slack message permalink into (channel_id, message_ts, thread_ts).

    URL formats:
      .../archives/C12345678/p1741234567123456
      .../archives/C12345678/p1741234567123456?thread_ts=1741234560.123456&cid=C12345678
    """
    parsed = urlparse(url)
    parts = [p for p in parsed.path.split("/") if p]

    try:
        archives_idx = parts.index("archives")
        channel_id = parts[archives_idx + 1]
        raw_ts = parts[archives_idx + 2]
    except (ValueError, IndexError):
        raise click.ClickException(f"Unrecognized Slack URL: {url}\nExpected .../archives/<channel>/<message>")

    if not raw_ts.startswith("p") or len(raw_ts) < 12:
        raise click.ClickException(f"Unrecognized message timestamp format: {raw_ts!r}")

    digits = raw_ts[1:]
    message_ts = digits[:10] + "." + digits[10:]

    qs = parse_qs(parsed.query)
    thread_ts = qs.get("thread_ts", [None])[0]

    return channel_id, message_ts, thread_ts


def parse_time(value: str) -> float:
    """
    Parse a human-friendly time into a Unix timestamp.

    Accepts:
      Relative: 30m, 3h, 2d, 1w
      Absolute: 2026-03-01, 2026-03-01T14:00:00
      Raw:      1741234567
    """
    now = datetime.now(timezone.utc)

    m = re.fullmatch(r"(\d+)([mhdw])", value)
    if m:
        n, unit = int(m.group(1)), m.group(2)
        delta = {"m": timedelta(minutes=n), "h": timedelta(hours=n), "d": timedelta(days=n), "w": timedelta(weeks=n)}[
            unit
        ]
        return (now - delta).timestamp()

    for fmt in ("%Y-%m-%dT%H:%M:%S", "%Y-%m-%d %H:%M:%S", "%Y-%m-%d"):
        try:
            dt = datetime.strptime(value, fmt).replace(tzinfo=timezone.utc)
            return dt.timestamp()
        except ValueError:
            pass

    try:
        return float(value)
    except ValueError:
        raise click.ClickException(
            f"Cannot parse time: {value!r}\n"
            "Use: 30m, 3h, 2d, 1w, 2026-03-01, 2026-03-01T14:00:00, or unix timestamp."
        )


def format_message(msg: dict) -> dict:
    out: dict = {}

    if ts := msg.get("ts"):
        out["ts"] = ts
        out["time"] = datetime.fromtimestamp(float(ts), tz=timezone.utc).strftime("%Y-%m-%d %H:%M UTC")
    if user := msg.get("user"):
        out["user"] = user
    if text := (msg.get("text") or "").strip():
        out["text"] = text[:500]
    if msg.get("reply_count"):
        out["reply_count"] = msg["reply_count"]
    if msg.get("reactions"):
        out["reactions"] = [f"{r['name']}({r['count']})" for r in msg["reactions"]]

    # Structured attachments (e.g. alert bots)
    if msg.get("attachments"):
        att = msg["attachments"][0]

        def _action_url(keyword: str) -> str | None:
            for a in att.get("actions") or []:
                if re.search(keyword, a.get("text", ""), re.I):
                    return a.get("url")
            return None

        attachment = {
            k: v
            for k, v in {
                "title": att.get("title"),
                "text": ((att.get("text") or att.get("fallback") or "").strip()[:300]) or None,
                "color": att.get("color"),
                "source": _action_url("source"),
                "silence": _action_url("silence"),
                "playbook": _action_url("playbook"),
            }.items()
            if v
        }
        if attachment:
            out["attachment"] = attachment

    return out


def build_permalink(team_url: str, channel_id: str, ts: str) -> str:
    """Construct a Slack message permalink from team URL, channel ID, and message timestamp."""
    ts_compact = ts.replace(".", "")
    base = team_url.rstrip("/")
    return f"{base}/archives/{channel_id}/p{ts_compact}"


def print_json(data) -> None:
    click.echo(json.dumps(data, indent=2, ensure_ascii=False))
