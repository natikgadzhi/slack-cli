import os
from pathlib import Path

SLACK_API = "https://slack.com/api"

USER_CACHE_PATH = Path(os.environ.get("SLACK_USER_CACHE", Path.home() / ".cache" / "slack-users.json"))
KC_ACCOUNT = os.environ.get("SLACK_KEYCHAIN_ACCOUNT", "natikgadzhi")
KC_XOXC_SERVICE = os.environ.get("SLACK_XOXC_SERVICE", "slack-xoxc-token")
KC_XOXD_SERVICE = os.environ.get("SLACK_XOXD_SERVICE", "slack-xoxd-token")

_USER_AGENT = (
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
    "AppleWebKit/537.36 (KHTML, like Gecko) "
    "Chrome/124.0.0.0 Safari/537.36"
)
