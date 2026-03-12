from unittest.mock import patch

from click.testing import CliRunner

from slack_cli.commands.check import check


XOXC = "xoxc-test-token-abc"
XOXD = "xoxd-test-cookie-xyz"
XOXD_ENCODED = "xoxd-test%2Bcookie-xyz"  # '+' encoded as %2B
XOXD_DECODED = "xoxd-test+cookie-xyz"


def _auth_ok(user="natik", team="Lambda"):
    return {"ok": True, "user": user, "team": team}


def _auth_fail(error="invalid_auth"):
    return {"ok": False, "error": error}


def test_check_success():
    runner = CliRunner()
    with (
        patch("slack_cli.commands.check.get_xoxc", return_value=XOXC),
        patch("slack_cli.commands.check.get_xoxd", return_value=XOXD),
        patch("slack_cli.commands.check._api_call_raw", return_value=_auth_ok()),
    ):
        result = runner.invoke(check)

    assert result.exit_code == 0
    assert "[OK]" in result.output
    assert "authenticated as natik on Lambda" in result.output
    assert "message, history, search" in result.output
    assert "[OK]" in result.output


def test_check_auth_failure():
    runner = CliRunner()
    with (
        patch("slack_cli.commands.check.get_xoxc", return_value=XOXC),
        patch("slack_cli.commands.check.get_xoxd", return_value=XOXD),
        patch("slack_cli.commands.check._api_call_raw", return_value=_auth_fail()),
    ):
        result = runner.invoke(check)

    assert "[FAIL]" in result.output
    assert "tokens expired" in result.output


def test_check_warns_on_unexpected_xoxc_prefix():
    runner = CliRunner()
    with (
        patch("slack_cli.commands.check.get_xoxc", return_value="xoxb-bot-token"),
        patch("slack_cli.commands.check.get_xoxd", return_value=XOXD),
        patch("slack_cli.commands.check._api_call_raw", return_value=_auth_ok()),
    ):
        result = runner.invoke(check)

    assert "[WARN]" in result.output
    assert "unexpected prefix" in result.output


def test_check_warns_on_bearer_prefix_in_token():
    runner = CliRunner()
    with (
        patch("slack_cli.commands.check.get_xoxc", return_value=f"Bearer {XOXC}"),
        patch("slack_cli.commands.check.get_xoxd", return_value=XOXD),
        patch("slack_cli.commands.check._api_call_raw", return_value=_auth_ok()),
    ):
        result = runner.invoke(check)

    assert "Bearer" in result.output
    assert "stripped" in result.output


def test_check_urldecoded_xoxd_fallback_succeeds():
    """When stored xoxd is URL-encoded and fails, check retries with decoded value."""
    runner = CliRunner()
    call_count = 0

    def fake_api(endpoint, xoxc, xoxd, **kwargs):
        nonlocal call_count
        call_count += 1
        if xoxd == XOXD_ENCODED:
            return _auth_fail()
        if xoxd == XOXD_DECODED:
            return _auth_ok()
        return _auth_fail()

    with (
        patch("slack_cli.commands.check.get_xoxc", return_value=XOXC),
        patch("slack_cli.commands.check.get_xoxd", return_value=XOXD_ENCODED),
        patch("slack_cli.commands.check._api_call_raw", side_effect=fake_api),
    ):
        result = runner.invoke(check)

    assert call_count == 2
    assert "url-decoded" in result.output
    assert "HINT" in result.output


def test_check_urldecoded_xoxd_fallback_also_fails():
    """When both encoded and decoded xoxd fail, reports full failure."""
    runner = CliRunner()

    def fake_api(endpoint, xoxc, xoxd, **kwargs):
        return _auth_fail()

    with (
        patch("slack_cli.commands.check.get_xoxc", return_value=XOXC),
        patch("slack_cli.commands.check.get_xoxd", return_value=XOXD_ENCODED),
        patch("slack_cli.commands.check._api_call_raw", side_effect=fake_api),
    ):
        result = runner.invoke(check)

    assert "tokens expired" in result.output
