import click

from slack_cli.auth import _keychain_set
from slack_cli.config import KC_ACCOUNT, KC_XOXC_SERVICE, KC_XOXD_SERVICE


@click.command("set-xoxc")
@click.argument("token")
def set_xoxc(token: str) -> None:
    """Save xoxc token to Keychain."""
    _keychain_set(KC_XOXC_SERVICE, token)
    click.echo(f"[OK] xoxc saved to Keychain (service={KC_XOXC_SERVICE!r}, account={KC_ACCOUNT!r})")


@click.command("set-xoxd")
@click.argument("token")
def set_xoxd(token: str) -> None:
    """Save xoxd cookie to Keychain."""
    _keychain_set(KC_XOXD_SERVICE, token)
    click.echo(f"[OK] xoxd saved to Keychain (service={KC_XOXD_SERVICE!r}, account={KC_ACCOUNT!r})")
