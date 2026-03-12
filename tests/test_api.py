import json
from io import BytesIO
from unittest.mock import MagicMock, patch
from urllib.error import HTTPError, URLError

import click
import pytest

from slack_cli.api import _api_call_raw


XOXC = "xoxc-test-token"
XOXD = "xoxd-test-cookie"


def _mock_response(data: dict, status: int = 200) -> MagicMock:
    resp = MagicMock()
    resp.read.return_value = json.dumps(data).encode()
    resp.__enter__ = lambda s: s
    resp.__exit__ = MagicMock(return_value=False)
    return resp


def test_successful_call():
    payload = {"ok": True, "user": "natik", "team": "Lambda"}
    with patch("slack_cli.api.urlopen", return_value=_mock_response(payload)):
        result = _api_call_raw("auth.test", xoxc=XOXC, xoxd=XOXD)
    assert result == payload


def test_passes_authorization_header():
    payload = {"ok": True}
    captured = {}

    def fake_urlopen(req):
        captured["auth"] = req.get_header("Authorization")
        return _mock_response(payload)

    with patch("slack_cli.api.urlopen", side_effect=fake_urlopen):
        _api_call_raw("auth.test", xoxc=XOXC, xoxd=XOXD)

    assert captured["auth"] == f"Bearer {XOXC}"


def test_passes_cookie_header():
    payload = {"ok": True}
    captured = {}

    def fake_urlopen(req):
        captured["cookie"] = req.get_header("Cookie")
        return _mock_response(payload)

    with patch("slack_cli.api.urlopen", side_effect=fake_urlopen):
        _api_call_raw("auth.test", xoxc=XOXC, xoxd=XOXD)

    assert captured["cookie"] == f"d={XOXD}"


def test_none_params_excluded():
    payload = {"ok": True}
    captured = {}

    def fake_urlopen(req):
        captured["body"] = req.data.decode()
        return _mock_response(payload)

    with patch("slack_cli.api.urlopen", side_effect=fake_urlopen):
        _api_call_raw("conversations.history", xoxc=XOXC, xoxd=XOXD, oldest=None, latest="12345")

    assert "oldest" not in captured["body"]
    assert "latest=12345" in captured["body"]


def test_http_error_raises_click_exception():
    error = HTTPError(url="https://slack.com/api/auth.test", code=429, msg="Too Many Requests", hdrs=None, fp=BytesIO(b"rate limited"))
    with patch("slack_cli.api.urlopen", side_effect=error):
        with pytest.raises(click.ClickException, match="HTTP 429"):
            _api_call_raw("auth.test", xoxc=XOXC, xoxd=XOXD)


def test_url_error_raises_click_exception():
    with patch("slack_cli.api.urlopen", side_effect=URLError("connection refused")):
        with pytest.raises(click.ClickException, match="Request failed"):
            _api_call_raw("auth.test", xoxc=XOXC, xoxd=XOXD)
