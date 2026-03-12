from urllib.parse import unquote

import click

from slack_cli.api import _api_call_raw
from slack_cli.auth import _sanitize_token, get_xoxc, get_xoxd


def _check_token(label: str, raw: str, expected_prefix: str) -> str:
    """Print diagnostics for a token and return the sanitized value."""
    clean, warnings = _sanitize_token(raw)

    issues: list[str] = []
    if warnings:
        issues += [f"[WARN] {label}: copy-paste artifact detected — {w}" for w in warnings]
    if not clean.startswith(expected_prefix):
        issues.append(f"[WARN] {label}: unexpected prefix, got {clean[:16]!r} (expected {expected_prefix!r})")

    if issues:
        for line in issues:
            click.echo(line)
        click.echo(f"       {label} ({len(clean)} chars): {clean[:20]}...")
    else:
        click.echo(f"[OK]   {label} ({len(clean)} chars): {clean[:20]}...")

    return clean


@click.command()
def check() -> None:
    """Verify tokens and API connectivity.

    Tests xoxc and xoxd separately, then together, so you know exactly
    which token is stale and which commands will work.

    \b
    Commands that always work (no auth):
      set-xoxc, set-xoxd

    \b
    Commands that require xoxc + xoxd:
      message, history, search
    """
    xoxc = _check_token("xoxc", get_xoxc(), "xoxc-")
    xoxd = _check_token("xoxd", get_xoxd(), "xoxd-")
    click.echo()

    both = _api_call_raw("auth.test", xoxc=xoxc, xoxd=xoxd)
    if both.get("ok"):
        click.echo(f"[OK]   xoxc + xoxd: authenticated as {both['user']} on {both['team']}")
        both_valid = True
    else:
        click.echo(f"[FAIL] xoxc + xoxd: {both.get('error')}")
        # Also try with URL-decoded xoxd in case it was stored encoded
        xoxd_decoded = unquote(xoxd)
        if xoxd_decoded != xoxd:
            both_decoded = _api_call_raw("auth.test", xoxc=xoxc, xoxd=xoxd_decoded)
            if both_decoded.get("ok"):
                click.echo(f"[OK]   xoxc + xoxd (url-decoded): authenticated as {both_decoded['user']} on {both_decoded['team']}")
                click.echo("[HINT] Your xoxd was stored URL-encoded. Re-save it decoded: slack set-xoxd <decoded-value>")
                both_valid = True
            else:
                click.echo(f"[FAIL] xoxc + xoxd (url-decoded): {both_decoded.get('error')}")
                both_valid = False
        else:
            both_valid = False

    click.echo()
    click.echo("Commands:")
    click.echo("  [OK]   set-xoxc, set-xoxd, check  (no auth required)")
    if both_valid:
        click.echo("  [OK]   message, history, search  (xoxc + xoxd: working)")
    else:
        click.echo("  [FAIL] message, history, search  (tokens expired — run: slack set-xoxc <token> && slack set-xoxd <cookie>)")
