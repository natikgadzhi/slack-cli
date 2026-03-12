import click

from slack_cli.api import api_call, get_team_url
from slack_cli.channels import resolve_channel
from slack_cli.formatting import build_permalink, format_message, parse_time, print_json
from slack_cli.users import resolve_users


@click.command()
@click.argument("channel")
@click.option("--since", metavar="TIME", default=None, help="Oldest message time: 30m, 3h, 2d, 1w, 2026-03-01, or unix ts")
@click.option("--until", metavar="TIME", default=None, help="Latest message time (same formats as --since); defaults to now")
@click.option("--limit", metavar="N", default=50, show_default=True, help="Max messages to return")
def channel(channel: str, since: str | None, until: str | None, limit: int) -> None:
    """Fetch channel message history.

    CHANNEL can be a name (e.g. general) or ID (e.g. C12345678). # prefix is accepted.

    \b
    Examples:
      slack channel general --since 2d
      slack channel '#incidents' --since 2026-03-01 --until 2026-03-10 --limit 100
      slack channel C12345678 --since 3h
    """
    channel_id = resolve_channel(channel)
    oldest = parse_time(since) if since else None
    latest = parse_time(until) if until else None

    resp = api_call(
        "conversations.history",
        channel=channel_id,
        limit=limit,
        oldest=oldest,
        latest=latest,
    )
    team_url = get_team_url()
    messages = resolve_users(resp.get("messages", []))
    result = []
    for m in messages:
        formatted = format_message(m)
        if ts := m.get("ts"):
            formatted["link"] = build_permalink(team_url, channel_id, ts)
        result.append(formatted)
    print_json(result)
