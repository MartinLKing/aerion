# My changes

Notes on what this personal fork changes compared with upstream Aerion.

## Feature 1: Own OAuth credentials (Gmail and Outlook)

No code changes. Aerion can already use your own credentials, so these are saved in the app under Settings → Accounts → OAuth credentials, not in the repo. They are tied to this Windows profile and have to be entered again on another PC.

**Running the app:** `wails dev -tags webkit2_41`

**Google Cloud**
- OAuth client of type Desktop app, with the People API enabled.
- The consent screen is still in Testing, so Gmail may need signing in again about every 7 days. Publishing requires home page, privacy policy and terms links.
- Calendar uses its own credentials slot, so it needs separate setup (feature 8).

**Azure app registration**
- Supported account types must include personal Microsoft accounts.
- Redirect URI under Mobile and desktop applications: `http://localhost/callback`. The `/callback` path is required, and Microsoft ignores the port.
- No client secret.

**Tooling:** Go 1.25.7 and Wails CLI v2.16.0, matching upstream's CI. Wails is pinned to the version in `go.mod`, because upstream notes that `@latest` broke their release by requiring a newer Go.

**Generated files:** `wails dev` rewrites `frontend/wailsjs/*`, `go.mod` and `frontend/package.json.md5`, so they show as modified in git. These are not real changes and should stay out of commits.
