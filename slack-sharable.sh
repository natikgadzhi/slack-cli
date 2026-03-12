#!/usr/bin/env bash
# Purpose:    Slack API query tool — search, channels, message history
# Operations: READ ONLY
# Auth:       macOS Keychain — service: "slack-xoxc-token" (xoxc-), "slack-xoxd-token" (xoxd-), account: "YOURNAME"
#             When xoxp- lands: service: "slack-xoxp-token" — set SLACK_AUTH_MODE=xoxp
# GUPPI APPROVED: 2026-03-06
set -euo pipefail

SLACK_API="https://slack.com/api"
AUTH_MODE="${SLACK_AUTH_MODE:-session}"  # "session" (xoxc+xoxd) or "xoxp"

# Keychain config — override via env if your entries use different names
# Default account is "YOURNAME". Others: export SLACK_KEYCHAIN_ACCOUNT=yourname
_KC_ACCOUNT="${SLACK_KEYCHAIN_ACCOUNT:-YOURNAME}"
_KC_XOXC="${SLACK_XOXC_SERVICE:-slack-xoxc-token}"
_KC_XOXD="${SLACK_XOXD_SERVICE:-slack-xoxd-token}"
_KC_XOXP="${SLACK_XOXP_SERVICE:-slack-xoxp-token}"

_xoxc() {
  local v
  v=$(security find-generic-password -a "$_KC_ACCOUNT" -s "$_KC_XOXC" -w 2>/dev/null)
  [[ -n "$v" ]] && echo "$v" && return
  echo "ERROR: ${_KC_XOXC} not found in Keychain (account: ${_KC_ACCOUNT})" >&2; exit 1
}

_xoxd() {
  local v
  v=$(security find-generic-password -a "$_KC_ACCOUNT" -s "$_KC_XOXD" -w 2>/dev/null)
  [[ -n "$v" ]] && echo "$v" && return
  echo "ERROR: ${_KC_XOXD} not found in Keychain (account: ${_KC_ACCOUNT})" >&2; exit 1
}

_xoxp() {
  local v
  v=$(security find-generic-password -a "$_KC_ACCOUNT" -s "$_KC_XOXP" -w 2>/dev/null)
  [[ -n "$v" ]] && echo "$v" && return
  echo "ERROR: ${_KC_XOXP} not found in Keychain (account: ${_KC_ACCOUNT})" >&2; exit 1
}

USER_CACHE="${SLACK_USER_CACHE:-$HOME/h/.cache/slack-users.json}"

# Resolve a Slack user ID to display name. Caches to $USER_CACHE.
_resolve_user() {
  local uid="$1"
  [[ -z "$uid" ]] && echo "unknown" && return
  # Check cache
  if [[ -f "$USER_CACHE" ]]; then
    local cached
    cached=$(python3 -c "import json,sys; d=json.load(open('$USER_CACHE')); print(d.get(sys.argv[1],''))" "$uid" 2>/dev/null)
    [[ -n "$cached" ]] && echo "$cached" && return
  fi
  # Fetch from API
  local name
  name=$(_call "users.info" --data-urlencode "user=${uid}" | jq -r '.user | .real_name // .name // empty' 2>/dev/null)
  [[ -z "$name" ]] && name="$uid"
  # Write to cache
  python3 - "$uid" "$name" "$USER_CACHE" <<'EOF'
import json, sys, os
uid, name, path = sys.argv[1], sys.argv[2], sys.argv[3]
d = {}
if os.path.exists(path):
    try: d = json.load(open(path))
    except: pass
d[uid] = name
json.dump(d, open(path, 'w'), indent=2)
EOF
  echo "$name"
}

# Resolve all user IDs in a JSON stream.
# Cache misses trigger a users.info API call and update the cache.
_resolve_users_in_json() {
  local data="$1"
  # Extract unique unknown UIDs
  local uids
  uids=$(echo "$data" | grep -oE '"user": "U[A-Z0-9]+"' | grep -oE 'U[A-Z0-9]+' | sort -u || true)
  for uid in $uids; do
    if [[ -f "$USER_CACHE" ]] && python3 -c "import json,sys; d=json.load(open('$USER_CACHE')); sys.exit(0 if sys.argv[1] in d else 1)" "$uid" 2>/dev/null; then
      continue  # already cached
    fi
    # Fetch and cache
    local name
    name=$(_call "users.info" --data-urlencode "user=${uid}" 2>/dev/null | jq -r '.user | .real_name // .name // empty' 2>/dev/null)
    [[ -z "$name" ]] && name="$uid"
    mkdir -p "$(dirname "$USER_CACHE")"
    python3 -c "
import json, os
path='$USER_CACHE'
d = {}
if os.path.exists(path):
    try: d = json.load(open(path))
    except: pass
d['$uid'] = '$name'
json.dump(d, open(path, 'w'), indent=2)
"
  done
  # Replace UIDs with resolved names. python3 -c reads code inline; cache path via argv, data via stdin.
  echo "$data" | python3 -c '
import json, sys, re, os
cache_path = sys.argv[1] if len(sys.argv) > 1 else ""
cache = {}
if cache_path and os.path.exists(cache_path):
    try: cache = json.load(open(cache_path))
    except: pass
def resolve(uid): return cache.get(uid, uid)
data = sys.stdin.read()
def replace_uid(m): return "\"user\": \"" + resolve(m.group(1)) + "\""
print(re.sub(r"\"user\":\s*\"(U[A-Z0-9]+)\"", replace_uid, data))
' "$USER_CACHE"
}

_call() {
  local endpoint="$1"; shift
  local extra_args=("$@")
  if [[ "$AUTH_MODE" == "xoxp" ]]; then
    curl -sf -X POST \
      -H "Authorization: Bearer $(_xoxp)" \
      -H "Content-Type: application/x-www-form-urlencoded; charset=utf-8" \
      "${extra_args[@]+"${extra_args[@]}"}" \
      "${SLACK_API}/${endpoint}"
  else
    curl -sf -X POST \
      -H "Authorization: Bearer $(_xoxc)" \
      -H "Cookie: d=$(_xoxd)" \
      -H "Content-Type: application/x-www-form-urlencoded; charset=utf-8" \
      "${extra_args[@]+"${extra_args[@]}"}" \
      "${SLACK_API}/${endpoint}"
  fi
}

_usage() {
  cat >&2 <<EOF
Usage: ~/h/bin/slack <command> [args]

Commands:
  check                          Test keychain + API connectivity
  search <query> [--count N]     Search messages across workspace (default: 20)
  channel-id <name>              Look up channel ID by name
  history <channel-id> [N]       Channel history with alerts+threads (default: 30)
  thread <channel-id> <ts>       Read thread replies
  channels                       List joined channels (paginated)

Auth mode: SLACK_AUTH_MODE=xoxp ~/h/bin/slack <cmd>  (when xoxp- token exists)

Examples:
  ~/h/bin/slack channel-id eng_observability_alerts
  ~/h/bin/slack history C073JA5JMJ9 50
  ~/h/bin/slack thread C073JA5JMJ9 1772812949.782749
  ~/h/bin/slack search "prometheus reload" --count 10
EOF
  exit 1
}

# jq filter: render a message with alert attachment, reactions, and thread indicator
_MSG_FILTER='
  .messages[] |
  {
    ts,
    thread_ts,
    reply_count,
    reactions: (if .reactions then [.reactions[] | "\(.name)(\(.count))"] else null end),
    user,
    text: (if .text != "" then .text[:300] else null end),
    alert: (.attachments[0] | if . then {
      title,
      text: ((.text // .fallback // "")[:300]),
      color,
      source: (.actions // [] | map(select(.text | test("Source";"i"))) | first | .url),
      silence: (.actions // [] | map(select(.text | test("Silence";"i"))) | first | .url),
      playbook: (.actions // [] | map(select(.text | test("Playbook";"i"))) | first | .url)
    } else null end)
  } | select(.text or .alert)
'

cmd="${1:-}"
shift || true

case "$cmd" in
  check)
    if [[ "$AUTH_MODE" == "xoxp" ]]; then
      _check_v=$(security find-generic-password -a "$_KC_ACCOUNT" -s "$_KC_XOXP" -w 2>/dev/null)
      if [[ -n "$_check_v" ]]; then echo "[OK] keychain: ${_KC_XOXP} found"
      else echo "[FAIL] keychain: ${_KC_XOXP} missing or empty (account: ${_KC_ACCOUNT})"; exit 1; fi
    else
      _vc=$(security find-generic-password -a "$_KC_ACCOUNT" -s "$_KC_XOXC" -w 2>/dev/null)
      _vd=$(security find-generic-password -a "$_KC_ACCOUNT" -s "$_KC_XOXD" -w 2>/dev/null)
      if [[ -n "$_vc" ]]; then echo "[OK] keychain: ${_KC_XOXC} found"
      else echo "[FAIL] keychain: ${_KC_XOXC} missing or empty (account: ${_KC_ACCOUNT})"; exit 1; fi
      if [[ -n "$_vd" ]]; then echo "[OK] keychain: ${_KC_XOXD} found"
      else echo "[FAIL] keychain: ${_KC_XOXD} missing or empty (account: ${_KC_ACCOUNT})"; exit 1; fi
    fi
    result=$(_call "auth.test")
    ok=$(echo "$result" | jq -r '.ok')
    if [[ "$ok" == "true" ]]; then
      user=$(echo "$result" | jq -r '.user')
      team=$(echo "$result" | jq -r '.team')
      echo "[OK] api: authenticated as ${user} on ${team}"
    else
      err=$(echo "$result" | jq -r '.error')
      echo "[FAIL] api: ${err}"
      exit 1
    fi
    ;;

  search)
    [[ -z "${1:-}" ]] && _usage
    query="$1"; shift
    count=20
    while [[ $# -gt 0 ]]; do
      case "$1" in
        --count) count="$2"; shift 2 ;;
        *) _usage ;;
      esac
    done
    _call "search.messages" \
      --data-urlencode "query=${query}" \
      --data-urlencode "count=${count}" \
      | jq '.messages.matches[] | {ts, channel: .channel.name, user, text}'
    ;;

  channel-id)
    [[ -z "${1:-}" ]] && _usage
    name="$1"
    _call "search.messages" \
      --data-urlencode "query=in:#${name} *" \
      --data-urlencode "count=1" \
      | jq '.messages.matches[0].channel | {id, name}'
    ;;

  history)
    [[ -z "${1:-}" ]] && _usage
    channel="$1"
    limit="${2:-30}"
    _raw=$(_call "conversations.history" \
      --data-urlencode "channel=${channel}" \
      --data-urlencode "limit=${limit}" \
      | jq "$_MSG_FILTER")
    _resolve_users_in_json "$_raw"
    ;;

  thread)
    [[ -z "${1:-}" || -z "${2:-}" ]] && _usage
    channel="$1"
    ts="$2"
    _raw=$(_call "conversations.replies" \
      --data-urlencode "channel=${channel}" \
      --data-urlencode "ts=${ts}" \
      | jq "$_MSG_FILTER")
    _resolve_users_in_json "$_raw"
    ;;

  channels)
    cursor=""
    while true; do
      resp=$(_call "conversations.list" \
        --data-urlencode "limit=200" \
        --data-urlencode "exclude_archived=true" \
        --data-urlencode "types=public_channel,private_channel,mpim,im" \
        ${cursor:+--data-urlencode "cursor=${cursor}"})
      echo "$resp" | jq '.channels[] | {id, name}'
      cursor=$(echo "$resp" | jq -r '.response_metadata.next_cursor // ""')
      [[ -z "$cursor" ]] && break
    done
    ;;

  search)
    [[ -z "${1:-}" ]] && _usage
    query="$1"; shift
    count=20
    while [[ $# -gt 0 ]]; do
      case "$1" in
        --count) count="$2"; shift 2 ;;
        *) _usage ;;
      esac
    done
    _call "search.messages" \
      --data-urlencode "query=${query}" \
      --data-urlencode "count=${count}" \
      | jq '.messages.matches[] | {ts, channel: .channel.name, user, text: .text[:300]}'
    ;;

  "")
    _usage
    ;;

  *)
    _usage
    ;;
esac
