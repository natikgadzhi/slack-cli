import json

from slack_cli.api import api_call
from slack_cli.config import USER_CACHE_PATH


def _load_user_cache() -> dict:
    if USER_CACHE_PATH.exists():
        try:
            return json.loads(USER_CACHE_PATH.read_text())
        except Exception:
            pass
    return {}


def _save_user_cache(cache: dict) -> None:
    USER_CACHE_PATH.parent.mkdir(parents=True, exist_ok=True)
    USER_CACHE_PATH.write_text(json.dumps(cache, indent=2))


def _fetch_username(uid: str) -> str:
    try:
        data = api_call("users.info", user=uid)
        user = data["user"]
        return user.get("real_name") or user.get("name") or uid
    except SystemExit:
        return uid


def resolve_users(messages: list) -> list:
    """Replace user IDs with display names in a list of message dicts."""
    cache = _load_user_cache()
    dirty = False

    unknown = {m["user"] for m in messages if m.get("user") and m["user"] not in cache}
    for uid in unknown:
        cache[uid] = _fetch_username(uid)
        dirty = True

    if dirty:
        _save_user_cache(cache)

    result = []
    for msg in messages:
        m = dict(msg)
        if m.get("user"):
            m["user"] = cache.get(m["user"], m["user"])
        result.append(m)
    return result
