import click

from slack_cli.api import api_call
from slack_cli.formatting import format_message, parse_slack_url, print_json
from slack_cli.users import resolve_users


@click.command()
@click.argument("url")
def message(url: str) -> None:
    """Fetch a message or full thread from a Slack URL.

    Fetches the whole thread if the URL points to a reply or a root message
    with replies. Otherwise returns just the single message.

    \b
    Examples:
      slack message 'https://myteam.slack.com/archives/C12345678/p1741234567123456'
      slack message 'https://myteam.slack.com/archives/C12345678/p1741234567123456?thread_ts=1741234560.000100'
    """
    channel_id, message_ts, thread_ts = parse_slack_url(url)

    if thread_ts:
        resp = api_call("conversations.replies", channel=channel_id, ts=thread_ts, limit=200)
        messages = resolve_users(resp.get("messages", []))
        print_json([format_message(m) for m in messages])
        return

    resp = api_call("conversations.replies", channel=channel_id, ts=message_ts, limit=200)
    messages = resp.get("messages", [])

    if len(messages) > 1:
        messages = resolve_users(messages)
        print_json([format_message(m) for m in messages])
    else:
        messages = resolve_users(messages)
        print_json(format_message(messages[0]) if messages else {})
