import click

from slack_cli.api import api_call
from slack_cli.formatting import format_message, parse_slack_url, print_json
from slack_cli.users import resolve_users


@click.command()
@click.argument("url")
def message(url: str) -> None:
    """Fetch a message or full thread from a Slack URL.

    If the URL is a thread reply (?thread_ts=...), fetches the whole thread.
    If the URL points to a root message with replies, fetches the whole thread.
    Otherwise returns just the single message.
    """
    channel_id, message_ts, thread_ts = parse_slack_url(url)

    # thread_ts in the URL means it's a reply — fetch the whole thread rooted at thread_ts
    if thread_ts:
        resp = api_call("conversations.replies", channel=channel_id, ts=thread_ts, limit=200)
        messages = resolve_users(resp.get("messages", []))
        print_json([format_message(m) for m in messages])
        return

    # No thread_ts — look at the message itself to decide
    resp = api_call("conversations.replies", channel=channel_id, ts=message_ts, limit=200)
    messages = resp.get("messages", [])

    if len(messages) > 1:
        messages = resolve_users(messages)
        print_json([format_message(m) for m in messages])
    else:
        messages = resolve_users(messages)
        print_json(format_message(messages[0]) if messages else {})
