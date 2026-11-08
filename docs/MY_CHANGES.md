# My changes

Notes on what this personal fork changes compared with upstream Aerion.

**Emailio** is a personal fork of [Aerion](https://github.com/hkdb/aerion), the open source email client by hkdb, licensed under Apache 2.0 (see `LICENSE`). This file and the git history record what was changed.

## Renamed to Emailio

Only the name users see changed, and the About tab credits Aerion.

Renamed:
- The window and title bar, the startup splash, and the composer window title.
- The program name (`wails build` produces `emailio.exe`, dev runs `emailio-dev.exe`), and the product details in `wails.json`.
- Startup error dialogs, the notification app name, and the English labels that refer to the app itself, including in the calendar and contacts extensions.

Deliberately kept as Aerion:
- The data folder `%APPDATA%\Aerion`, the database, and the "aerion" Windows Credential Manager entries. Renaming these would disconnect the accounts and saved logins.
- Go module and package names, internal IDs, the IMAP client ID sent to servers, and environment variables such as `AERION_LOG_LEVEL`, so upstream merges stay easy.
- Labels that really mean Aerion: its terms of use, its official builds and built-in OAuth credentials ("Aerion - Google"), Flatpak notes, and upstream's What's New text.
- The icon, and the other languages' labels.

## Folder management

The ⋮ menu of each account has "New folder", and right-clicking one of the user's own folders gives "New subfolder", "Rename" and "Delete folder". Changes are made on the mail server, so they appear in webmail and on other devices. Tested on Gmail (where folders are labels) and Outlook.

- System folders (Inbox, Sent, Drafts, Trash, Spam, Archive and similar) can't be renamed or deleted.
- Delete is refused when the folder still holds mail (checked live on the server, since some servers delete the messages with the folder), has subfolders, or a filter rule files into it.
- Rename updates the local folder records in place before refreshing, so folder IDs, and the filter rules that use them, survive. Subfolders move with their parent.
- New: `app/folder_manage.go` (with tests for the path logic), `internal/imap/mailbox_manage.go`, `FolderNameDialog.svelte`. Changed: `FolderContextMenu.svelte`, `FolderTreeItem.svelte`, `AccountSection.svelte`.

## Fix: Microsoft contacts sign-in

- "Sign in with Microsoft" in Settings → Contacts did nothing (a quick flicker). The contacts-only sign-in only used credentials compiled into the app, so credentials saved in Settings were ignored. It now uses the same resolved credentials as token refresh (`StartContactsOnlyOAuthFlow` in `app/carddav.go`). Upstream has the same bug.
- The Microsoft tab no longer offers to link an existing mail account. That reuses the mail token, which is issued for Outlook IMAP and can't be read by Microsoft Graph, so contacts sync failed with 401 InvalidAuthenticationToken.

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

## Feature 5: Filter rules

Settings has a new Rules tab. A rule matches on sender (contains, equals, in domain) and/or subject (contains, equals), then archives or moves the message and/or marks it read. Rules can cover all accounts or one, and moving to a folder needs a specific account. The Settings dialog was widened to fit nine tabs.

- New mail: rules run on newly arrived Inbox messages after each sync, before any desktop notification, and the first matching rule wins. There's no notification for mail the rules dealt with.
- Existing mail: the play button runs one rule on downloaded Inbox mail after showing a count. Mail that's only on the server isn't touched.
- Only runs while Aerion is running. Mail arriving while it's closed is filtered at the next sync.
- Which mail is new: each Inbox's highest checked UID and UIDVALIDITY are kept in the settings table. The first check only records the position, so adding a rule or account doesn't file existing mail.
- The last 2,000 filed Message-IDs are remembered, so a message moved back to the Inbox isn't filed again.
- Rule moves stay off the Undo list, using `moveToFolder(..., undoable)` in `actions.go`. Mark read and move together waits for the server to confirm the read flag before moving.
- A rule files single messages, so a thread can end up split between folders.
- New: `internal/rules/`, `internal/settings/filter_rules.go`, `internal/message/rule_candidates.go`, `app/rules.go`, `RulesTab.svelte`. No database migration; labels are English only.

## Feature 8: Google Calendar and "Add to calendar" from email

**Setup (no code):** Aerion's calendar extension already does two-way Google Calendar sync and event create/edit/delete. To connect it:
- Enable the Google Calendar API in the Google Cloud project.
- Enter the same OAuth client in the Calendar extension's own credentials slot (`google-calendar`, under Settings → Extensions → Calendar), choosing the custom option.
- Add the calendar from the Calendar view with Add calendar → Google.
- Google's 7-day sign-in limit in Testing mode applies here too.

**Add to calendar (code):** each expanded email has an "Add to calendar" link that opens the calendar's normal new-event form, prefilled:
- Title: the subject.
- Description: the sender, when it arrived, and the first 500 characters of the email.
- Date and time: found by `frontend/src/lib/calendar/detectEventDate.ts`, checking the subject, then the email text.

How the date finder works:
- Understands English, French, German and Spanish wording (12/11/2026, 12 novembre à 14h30, 12. November um 14 Uhr, 12 de noviembre, November 12 2:30 pm), and time ranges (9h à 11h, 14:00-15:30, von 14 bis 16 Uhr).
- Reads numeric dates day first. Numeric dates need a year, so version numbers aren't mistaken for dates.
- Prefers the first upcoming date. If every date has passed, it uses the first one and the form says so. A yearless date from the last 60 days counts as this year.
- Doesn't understand relative words like "tomorrow".

How it's wired up:
- `EventComposerDialog.svelte` (upstream calendar code) gained optional settings `initialSummary`, `initialDescriptionHTML`, `defaultEnd`, `defaultAllDay` and `notice`. Without them it behaves as before.
- `AddToCalendarButton.svelte` loads the calendar code only when clicked, and the link only appears when the Calendar extension is enabled.
- The date finder was tested with 40 phrasings in a scratch script. The frontend has no test runner, so those tests aren't in the repo.

## Feature 6: Translation (Azure Translator)

Emails, selected text and drafts can be translated with Azure Translator. Nothing is sent until you click.

- Settings → Composer → Translation: the Azure key (saved in the credential store, Windows Credential Manager), the region from Azure's Keys and Endpoint page, the default language (English, French, German, Spanish), and Test.
- Reading mail: "Translate to ..." above each message switches it in place, with formatting kept, and "Show original" switches back. The ▾ picks another language. Translations are kept until the app closes. Right-clicking selected text offers Translate, shown in a small panel with a copy button.
- Writing: Translate in the composer toolbar (rich text only) translates only the user's own text, leaving the signature and quoted text alone. Ctrl+Z undoes it.
- Encrypted and signed mail is never translated. Translated HTML goes through the email sanitizer before it's shown or inserted.
- Long emails: Azure takes 50,000 characters per request, so HTML over that is translated as plain text in pieces. The free tier is also paced at about 33,000 characters a minute, so very long emails can get "too many translations in a short time". Accepted as is for occasional use.
- DeepL was dropped: its free Developer plan forbids personal data and may store content indefinitely. Microsoft says Translator doesn't store text sent for translation.
- New: `internal/translate/` (client and tests), `app/translate.go`, `TranslationSettings.svelte`, `TranslationPopup.svelte`, `TranslateDraftButton.svelte`, `lib/translation/languages.ts`. `EditorToolbar.svelte` gained a `trailing` snippet slot.

## Feature 7: Spell-check language dropdown

The composer's From row has a spell-check language dropdown. Picking a language checks the draft against that dictionary only, so "Hello, je t'aime" gets underlined under English. "All enabled" keeps upstream's behavior, where a word passes if any language switched on in Settings accepts it. The choice is remembered, and English (US) is used until one is picked.

- Works in rich text, plain text and the pop-out composer, which never loaded spell-check settings before.
- Quoted text is no longer checked: blockquotes, "> " lines, the "On ... wrote:" line, and everything from a forwarded message header on.
- Added the Spanish dictionary (`dictionary-es`, a new npm dependency) to `locales.ts` and `copy-dictionaries.mjs`.
- Words added to the user dictionary are accepted in every language. The French dictionary itself accepts "Hello".
- New: `SpellLanguageSelect.svelte`, `internal/settings/spellcheck_language.go`, `app/spellcheck_language.go`.
- `wails generate module` writes nothing while Aerion is running, so close the app before regenerating bindings.

## Action bar for ticked messages

Ticking messages shows a bar at the top of the list with the count, Archive, Delete, Mark read/unread, Star/Unstar, Move to, Select all and Clear. Upstream only offers these through the right-click menu, keyboard shortcuts and drag.

- New: `frontend/src/lib/components/list/BulkActionBar.svelte`. It's shown from `MessageList.svelte` and uses the same backend actions as the right-click menu.
- Select all only ticks the messages loaded in the list. The bar doesn't appear for server search results, which may not be downloaded.
- Fixed a bug from feature 4: inside Trash, deleting an All accounts search result offered a permanent delete, though the result wasn't in Trash.

## Threads show newest first

Conversations display the newest message at the top and open scrolled to the top. Only the display order changed in `ConversationViewer.svelte`: the underlying list stays oldest-first, because other code (such as replying to the latest message) relies on that.
