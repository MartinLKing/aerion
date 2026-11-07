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

## Feature 2: Attachments always visible in conversations

Every message in a thread, collapsed or expanded, shows its attachments as chips in the message header, with name, size, and Open and Save buttons always visible. Messages with more than one attachment also get a Save all button. This replaces the old list at the bottom of expanded messages.

- Changed: `frontend/src/lib/components/viewer/AttachmentList.svelte` and `ConversationViewer.svelte`.
- Inline images (signature logos, embedded pictures) get no chip, since they show in the message body.
- Known quirk: Save all still includes inline images, because it uses the existing backend function.
- Encrypted mail only shows its attachments after the message is opened and decrypted.

## Threads show newest first

Conversations display the newest message at the top and open scrolled to the top. Only the display order changed in `ConversationViewer.svelte`: the underlying list stays oldest-first, because other code (such as replying to the latest message) relies on that.
