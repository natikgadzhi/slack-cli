import subprocess

import click

from slack_cli.config import KC_ACCOUNT, KC_XOXC_SERVICE, KC_XOXD_SERVICE


def _keychain_get(service: str) -> str:
    try:
        result = subprocess.run(
            ["security", "find-generic-password", "-a", KC_ACCOUNT, "-s", service, "-w"],
            capture_output=True,
            text=True,
            check=True,
        )
        return result.stdout.strip()
    except subprocess.CalledProcessError:
        raise click.ClickException(
            f"Keychain entry not found (service={service!r}, account={KC_ACCOUNT!r}).\n"
            f"  Store it with: security add-generic-password -a {KC_ACCOUNT!r} -s {service!r} -w <token>"
        )


def _keychain_set(service: str, token: str) -> None:
    # Delete any existing entry first (add fails if it already exists)
    subprocess.run(
        ["security", "delete-generic-password", "-a", KC_ACCOUNT, "-s", service],
        capture_output=True,
    )
    subprocess.run(
        ["security", "add-generic-password", "-a", KC_ACCOUNT, "-s", service, "-w", token],
        check=True,
    )


def _sanitize_token(token: str) -> tuple[str, list[str]]:
    """Strip common copy-paste artifacts from a token. Returns (clean, warnings)."""
    warnings: list[str] = []
    t = token.strip()

    if t != token:
        warnings.append("had leading/trailing whitespace — stripped")

    if (t.startswith('"') and t.endswith('"')) or (t.startswith("'") and t.endswith("'")):
        t = t[1:-1]
        warnings.append("had surrounding quotes — stripped")

    if t.lower().startswith("bearer "):
        t = t[7:]
        warnings.append('had "Bearer " prefix — stripped')

    return t, warnings


def get_xoxc() -> str:
    import os

    return os.environ.get("SLACK_XOXC") or _keychain_get(KC_XOXC_SERVICE)


def get_xoxd() -> str:
    import os

    # Pass verbatim — DevTools already gives the value in the correct encoding
    return os.environ.get("SLACK_XOXD") or _keychain_get(KC_XOXD_SERVICE)
