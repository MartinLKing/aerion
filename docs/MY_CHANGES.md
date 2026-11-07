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

## Feature 3: Multiple signatures

Settings has a new Signatures tab for named signatures that work across all accounts, with an optional default per account. The composer has a signature dropdown beside the From field.

- The starting signature is the account's default, or otherwise the address's own signature, and it follows that address's settings for when signatures are added.
- A signature picked by hand always inserts, and stays if the From address changes.
- The dropdown is hidden in plain text mode, which doesn't insert signatures. A reopened draft keeps its signature, but the dropdown can't tell which one it is.
- Saved as JSON in the existing settings table, not a new database table, so there's no migration to clash with upstream's numbered ones.
- New: `internal/settings/signatures.go`, `app/signatures.go`, `SignaturesTab.svelte`. New labels are English only.
- Fixed two existing bugs in `composerSignature.ts`, which also affected changing the From address: inserting a signature on a reply or forward discarded text typed above the quote, and removing one could delete quoted text containing an earlier Aerion signature. Upstream changes to these functions may need manual merging.

Bindings in `frontend/wailsjs` were committed for the new functions (`App`, `ComposerApp` and `models.ts`). Generated changes that come with a new backend function belong in the commit; the rest of what `wails dev` rewrites does not.

## Feature 4: Unified search across accounts

Searching from the Unified Inbox now covers every downloaded folder of every account, except Trash and Spam. Inside a folder, the search box has a pill that switches between All accounts (the default) and This folder. Results show the account's color dot and the folder name.

- New: `internal/message/search_all.go`, kept separate so upstream merges stay simple. The existing unified inbox search function remains in the backend but the list no longer calls it.
- A result opens from the folder showing the most of its thread: the inbox, then other folders, then Sent or Drafts. This is chosen from the whole thread, because opening from Sent would hide the thread's inbox messages.
- HTML-only mail now gets a plain text `body_text` when downloaded, using `email.ExtractPlainTextFromHTML`, because search only indexes that field. Existing mail was backfilled once at startup (the `html_body_text_backfilled` setting), skipping encrypted messages and drafts. Display still prefers the HTML.
- Search only covers what's downloaded. Sync settings are deliberately unchanged: Inbox, Sent, Drafts and Trash from the last 180 days. Shift+Enter inside a folder still searches that folder on the server.
- Known limit: when the match is deep in a body, the result preview may not show the searched word.

## Threads show newest first

Conversations display the newest message at the top and open scrolled to the top. Only the display order changed in `ConversationViewer.svelte`: the underlying list stays oldest-first, because other code (such as replying to the latest message) relies on that.
