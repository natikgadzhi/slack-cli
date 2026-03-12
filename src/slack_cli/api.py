import json
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode
from urllib.request import Request, urlopen

import click

from slack_cli.auth import get_xoxc, get_xoxd
from slack_cli.config import SLACK_API, _USER_AGENT


def _api_call_raw(endpoint: str, xoxc: str, xoxd: str, **params) -> dict:
    body = urlencode({k: v for k, v in params.items() if v is not None}).encode()
    req = Request(
        f"{SLACK_API}/{endpoint}",
        data=body,
        headers={
            "Authorization": f"Bearer {xoxc}",
            "Cookie": f"d={xoxd}",
            "Content-Type": "application/x-www-form-urlencoded; charset=utf-8",
            "User-Agent": _USER_AGENT,
        },
        method="POST",
    )
    try:
        with urlopen(req) as resp:
            return json.loads(resp.read())
    except HTTPError as e:
        body = e.read().decode(errors="replace")
        raise click.ClickException(f"HTTP {e.code} {e.reason} from {endpoint}: {body[:300]}")
    except URLError as e:
        raise click.ClickException(f"Request failed for {endpoint}: {e.reason}")


def api_call(endpoint: str, **params) -> dict:
    return _api_call_raw(endpoint, xoxc=get_xoxc(), xoxd=get_xoxd(), **params)


_team_url_cache: str | None = None


def get_team_url() -> str:
    """Return the workspace base URL (e.g. https://myteam.slack.com).

    Checks SLACK_TEAM_URL env var first, then fetches from auth.test.
    """
    import os

    global _team_url_cache
    if _team_url_cache:
        return _team_url_cache

    if url := os.environ.get("SLACK_TEAM_URL"):
        _team_url_cache = url.rstrip("/")
        return _team_url_cache

    data = api_call("auth.test")
    _team_url_cache = data.get("url", "").rstrip("/")
    return _team_url_cache
