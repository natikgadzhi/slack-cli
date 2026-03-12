import click

from slack_cli.api import api_call
from slack_cli.formatting import print_json


@click.command()
@click.argument("query")
@click.option("--count", metavar="N", default=20, show_default=True, help="Max results to return")
def search(query: str, count: int) -> None:
    """Search messages across the workspace."""
    resp = api_call("search.messages", query=query, count=count)
    matches = resp.get("messages", {}).get("matches", [])
    results = [
        {k: v for k, v in {
            "ts": m.get("ts"),
            "channel": m.get("channel", {}).get("name"),
            "user": m.get("username") or m.get("user"),
            "text": (m.get("text") or "").strip()[:500] or None,
            "permalink": m.get("permalink"),
        }.items() if v}
        for m in matches
    ]
    print_json(results)
