import click

from slack_cli.commands.auth_cmds import set_xoxc, set_xoxd
from slack_cli.commands.check import check


@click.group()
def auth() -> None:
    """Manage Slack authentication tokens."""


auth.add_command(check)
auth.add_command(set_xoxc)
auth.add_command(set_xoxd)
