import click

from slack_cli.api import api_call
from slack_cli.channels import resolve_channel
from slack_cli.formatting import format_message, parse_time, print_json
from slack_cli.users import resolve_users


@click.command()
@click.argument("channel")
@click.option("--since", metavar="TIME", default=None, help="Oldest message time: 30m, 3h, 2d, 1w, 2026-03-01, or unix ts")
@click.option("--until", metavar="TIME", default=None, help="Latest message time (same formats as --since); defaults to now")
@click.option("--limit", metavar="N", default=50, show_default=True, help="Max messages to return")
def history(channel: str, since: str | None, until: str | None, limit: int) -> None:
    """Fetch channel message history."""
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
    messages = resolve_users(resp.get("messages", []))
    print_json([format_message(m) for m in messages])
