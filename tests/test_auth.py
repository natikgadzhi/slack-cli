import pytest

from slack_cli.auth import _sanitize_token


def test_clean_token_unchanged():
    token = "xoxc-12345-abcde"
    clean, warnings = _sanitize_token(token)
    assert clean == token
    assert warnings == []


def test_strips_double_quotes():
    clean, warnings = _sanitize_token('"xoxc-12345"')
    assert clean == "xoxc-12345"
    assert any("quotes" in w for w in warnings)


def test_strips_single_quotes():
    clean, warnings = _sanitize_token("'xoxc-12345'")
    assert clean == "xoxc-12345"
    assert any("quotes" in w for w in warnings)


def test_strips_bearer_prefix():
    clean, warnings = _sanitize_token("Bearer xoxc-12345")
    assert clean == "xoxc-12345"
    assert any("Bearer" in w for w in warnings)


def test_strips_bearer_prefix_case_insensitive():
    clean, warnings = _sanitize_token("bearer xoxc-12345")
    assert clean == "xoxc-12345"
    assert any("Bearer" in w for w in warnings)


def test_strips_whitespace():
    clean, warnings = _sanitize_token("  xoxc-12345  ")
    assert clean == "xoxc-12345"
    assert any("whitespace" in w for w in warnings)


def test_strips_multiple_artifacts():
    clean, warnings = _sanitize_token('  "Bearer xoxc-12345"  ')
    assert clean == "xoxc-12345"
    assert len(warnings) >= 2
