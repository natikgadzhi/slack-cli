import re

import click

from slack_cli.api import api_call


def resolve_channel(name_or_id: str) -> str:
    """Return channel ID. Passes through IDs (C...) unchanged; resolves names via API."""
    name_or_id = name_or_id.lstrip("#")
    if re.match(r"^[A-Z0-9]{8,}$", name_or_id):
        return name_or_id

    cursor = None
    while True:
        resp = api_call(
            "conversations.list",
            limit=200,
            exclude_archived="true",
            types="public_channel,private_channel,mpim,im",
            **({"cursor": cursor} if cursor else {}),
        )
        for ch in resp.get("channels", []):
            if ch.get("name") == name_or_id:
                return ch["id"]
        cursor = resp.get("response_metadata", {}).get("next_cursor")
        if not cursor:
            break

    raise click.ClickException(f"Channel not found: {name_or_id!r}")
