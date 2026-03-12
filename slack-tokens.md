# Getting Slack session tokens (xoxc + xoxd)

Slack's browser session uses two tokens together:

- **xoxc** — authorization token, sent as a Bearer header
- **xoxd** — session cookie, sent as the `d` cookie

Both are required. They expire when you log out or after extended inactivity.

## Steps

1. Open Slack in your browser (not the desktop app) and log in to your workspace.

2. Open DevTools: `Cmd+Option+I` on Mac, `F12` on Windows/Linux.

3. **Get xoxd** (the cookie):
   - Go to the **Application** tab → **Cookies** → `https://app.slack.com`
   - Find the cookie named exactly **`d`** (not `d-s` or anything else)
   - Copy its value — it starts with `xoxd-`

   > The value shown in DevTools is URL-encoded. Copy it as-is; the script decodes it automatically.

4. **Get xoxc** (the token):
   - Go to the **Network** tab
   - Click around in Slack (open a channel) to trigger some API requests
   - Filter requests by `slack.com/api`
   - Click any request → **Headers** → find the `Authorization` request header
   - Copy the value after `Bearer ` — it starts with `xoxc-`

   Alternatively: **Application** → **Local Storage** → `https://app.slack.com` →
   find the key `localConfig_v2`, open the JSON, look for a `"token"` field.

5. Store both in macOS Keychain:

   ```sh
   security add-generic-password -a natikgadzhi -s slack-xoxc-token -w "xoxc-..."
   security add-generic-password -a natikgadzhi -s slack-xoxd-token -w "xoxd-..."
   ```

6. Verify:

   ```sh
   python3 slack.py check
   ```

## Refreshing tokens

If you start getting `invalid_auth` errors, your tokens have expired. Repeat steps 1–5:

```sh
security delete-generic-password -a natikgadzhi -s slack-xoxc-token
security delete-generic-password -a natikgadzhi -s slack-xoxd-token
# then add fresh ones
```
