import click

from slack_cli.commands import auth, history, message, search


@click.group()
def cli() -> None:
    """Slack read-only CLI for Claude and other tools.

    Auth: macOS Keychain (service slack-xoxc-token / slack-xoxd-token, account natikgadzhi)
    or env vars SLACK_XOXC and SLACK_XOXD.
    """


cli.add_command(auth)
cli.add_command(message)
cli.add_command(history)
cli.add_command(search)


def main() -> None:
    cli()
