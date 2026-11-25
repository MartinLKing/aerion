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
- The other languages' labels, and the Linux and macOS icons.

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

## Fix: received messages lost most of their attachments

Messages with several attachments showed only one or two of them, and inline images kept showing "Loading...". The files were sent and received in full; Emailio dropped them while saving the message. Attachment IDs were made from the current time, and on Windows the clock only moves every 0.5-15 ms, so attachments read in the same tick shared an ID and the database kept only the first. Upstream Aerion has the same bug; it rarely shows on Linux.

- `generateID` in `internal/sync/helpers.go` now returns a UUID. Test: `internal/sync/attachment_id_test.go`.
- Messages synced before the fix keep their missing attachments, since already-stored messages aren't re-read.

## Replied and forwarded arrows in the message list

A conversation row shows a reply arrow when you have answered its newest message, and a forward arrow when you have forwarded it. Upstream stores the server's replied/forwarded flags but never shows them, and never sets them when you send.

- The arrow reflects the newest message in the row, so it goes away again when a new, unanswered message arrives in the thread.
- A message counts as replied when the server flag says so, or when the account's Sent folder holds a message written in reply to it. The second check is needed because Gmail and Outlook don't reliably set the flag for replies sent from their own web and phone apps.
- A sent message that answers another one counts as a forward when its subject starts with Fw:, Fwd:, TR:, WG: or RV:; otherwise it is a reply.
- Sending a reply or forward from Emailio (main window or pop-out composer) flags the original straight away and passes the flag on to the mail server. Servers that refuse the forwarded keyword only log a warning; the arrow still shows from the Sent folder.
- New: `internal/message/reply_state.go` (with tests), `app/reply_state.go`. The list queries in `store.go` are untouched apart from one added call each; the state is looked up for the page being shown, not the whole folder.
- No database changes.

## Red exclamation on an account explains itself

The red exclamation next to an account name means the last sync, or loading the folder list, failed. Clicking it now opens a small popup with a plain-language summary, the raw error text from the mail server, and Try again / Edit Account buttons. The "Sync error. Try again..." line under an account opens the same popup.

- Upstream attaches the error as a hover tooltip, but the hidden "⋮" menu button sits on top of the icon and swallows the hover, so it never showed. The icon now sits to the left of that button.
- New: `frontend/src/lib/utils/accountError.ts` sorts errors into sign-in, certificate, connection, timeout or unknown from their wording, and picks one account's part out of a "sync all" error. The popup is in `AccountSection.svelte`.
- Frontend only; no backend changes.

## Fix: sent messages appeared twice in a conversation

Autosave sometimes left an old copy of a draft on the server. After sending, that copy stayed in Drafts, and since the conversation view includes Drafts, it showed up as a second copy of the sent message, with an earlier time and slightly older text. Messages were only ever sent once. It happened more with attachments or a slow server, because uploads take longer.

Three causes, all in how drafts are uploaded:

- When an autosave was replaced by a newer one mid-upload, the code tried to delete the copy it had just uploaded using an already-cancelled request, so the delete always failed. It now uses a fresh one.
- A new autosave could start before the previous upload had recorded where its copy went, so the old copy was never deleted. Uploads and deletes of the same draft now run one at a time, and each re-reads where the current copy is first. Saving the draft text also no longer writes back an out-of-date copy location.
- Deleting the draft after sending skipped the server copy whenever a newer autosave was still pending. It now deletes any recorded copy. A failed upload also keeps the record of the old copy instead of forgetting it.

- Changed: `app/draft.go` (shared by the main window and the pop-out composer), `internal/draft/store.go` (`Update` no longer writes the copy location; new `MarkSyncFailed`). Tests in `internal/draft/store_test.go`.
- Leftover drafts from before the fix stay until deleted by hand.

## Default email app setting

Settings → General → Default email app makes Emailio available in Windows Settings → Default apps, so it can be chosen for mailto links. The button registers the running copy of Emailio for the current Windows user and opens Default apps; you then pick Emailio for MAILTO there. Windows doesn't let an app make itself the default.

- Upstream's installer does the same registration for all users (needs admin). This writes it for the current user only (`HKEY_CURRENT_USER`), pointing at the exe that is running, so it works without the installer.
- Run it from the built `emailio.exe`, not dev mode, or Default apps will open the dev build. If you move the exe, open the setting again and use "Use this copy".
- "Remove" deletes the entries again.
- Only `mailto` for now. Opening `.ics` and `.eml` files would need Emailio to handle file arguments first.
- New: `internal/platform/mailclient_windows.go` (stub for other systems in `mailclient_other.go`), `app/default_mail_app.go`, `frontend/src/lib/components/settings/DefaultMailAppSection.svelte`.

## Open .ics calendar files

Opening an `.ics` file (an invitation, or one saved from a website) shows the event in the calendar's new-event form, filled in with title, date and time, location, organizer and description. Pick the calendar and press Save. This also works for `.ics` attachments opened from an email, and when Emailio is already running.

- Settings → General → Default email app now registers `.ics` as well as MAILTO. An existing registration shows "Update and open Default apps"; after updating, choose Emailio for `.ics` in Windows Default apps.
- Limits: only the first event in a file is used, a repeating event is added once, and nothing is sent back to the organizer (no accept/decline reply). The form's note says when the first two apply.
- All-day events: `.ics` files end them on the day after; the form's end date is the last day, so one day is taken off.
- Files over 1 MB are refused.
- New: `app/ics_open.go` (with tests, uses the calendar's existing `.ics` reader), `frontend/src/lib/components/calendar/IcsOpenHandler.svelte`. The event form gained an `initialLocation` prefill. The three single-instance files accept an `ics:<path>` command so a running Emailio can be handed a file.

## Edit and send drafts; separate-window composer for replies

**Editing a draft now edits that draft.** Opening a draft used to hand the composer the message's ID instead of the draft's, so the first autosave made a new draft and sending deleted nothing: the original stayed in Drafts forever. `OpenDraftForEdit` (`app/draft_edit.go`, with tests) now resolves the real draft. A draft written elsewhere (Gmail web, phone) is taken over: Emailio makes a local draft pointing at the copy on the server, so autosave replaces it and sending removes it. Its body and attachments are fetched if needed.

- The small pencil is now a labelled **Edit draft** button, shown on drafts in any conversation, not only in the Drafts folder.
- Double-clicking a conversation in the Drafts folder opens it for editing.

**Separate window.** With Settings → Composer → composer mode set to separate window, replies, reply-all, forwards and draft edits now open in their own window too. Upstream only did this for new messages, so replies always used the overlay that blocks the main window.

## Contacts from sent mail, suggestions by most recent

Recipient suggestions list the person you emailed last first; how often you email them only breaks ties, and people never emailed come last, alphabetically. Upstream sorts by how often, and cut the list to 10 before the final sort.

- Everyone you send to (To, Cc and now Bcc) is saved as a collected contact in Emailio only, not in Google or Microsoft contacts. Your own addresses are skipped and failures are logged.
- Recipients are also learned from the Sent folder after each Sent sync, with the real sent dates: all existing Sent mail the first time, then only newer mail. This covers mail sent from webmail or a phone. A marker per account in the settings table remembers how far it has read, so old mail doesn't bring back a contact you deleted. Emailing that person again does.
- An open Contacts view reloads after a send or when contacts were learned.
- New: `app/contact_collect.go`, `internal/contact/sent.go`, `internal/message/sent_recipients.go`, with tests. Changed: ranking in `internal/contact/store.go`. No database table changes.
- `TestMSSidecar_*` in `extensions/contacts/backend` fail on Windows with or without this change (a temp file is still open at cleanup).

## Save a contact from an email; recipient addresses always shown

Recipients in the composer show the name and the address, so it is clear which of someone's addresses is used. Each has an icon that saves the address as a contact or renames it; the same icon is next to the sender when reading.

- Outline icon: not saved yet (including automatically collected). Filled icon: saved, click to rename. Contacts from a synced address book count as saved and are not changed here. Own addresses have no icon.
- Saving a collected contact makes it a regular one and protects its name from later mail. Contacts stay in Emailio only.
- A name saved in the composer is also used in the email being written. A save in a pop-out composer refreshes the main window (`contacts_changed` IPC message).
- New: `internal/contact/saved.go` (with tests), `app/contact_save.go`, `SaveContactButton.svelte`, `contactSaves.svelte.ts`.
- Bindings can be generated while Emailio is open by pointing APPDATA at an empty folder for that command; the generator otherwise finds the running copy and writes nothing.

## Fix: some attachments were hidden

The attachment chips left out every file marked inline, assuming the body shows it. Apple Mail marks attached photos and PDFs as inline too, so they appeared nowhere although the list showed a paperclip.

- A file is now left out only if it is an image the message's HTML displays (and at most 5 MB, the limit for showing it). Everything else gets a chip, also in decrypted messages. Emails already synced are fixed without a re-sync.
- Save All saves and counts only the files shown as chips; it used to include logos.
- New: `frontend/src/lib/utils/inlineAttachments.ts`. `SaveAllAttachments` and `SaveAllEncryptedAttachments` now take the files to save.
- Known issue, not changed: files are opened and saved by name, so of two different files with the same name in one email, the first is used for both.

## Fix: wrong file when an email has several with the same name

Files were looked up in the email by name only, so opening or saving the second of two same-named files gave the first, and Save All kept only one.

- A file is now picked by Content-ID, then size, then position (`internal/email/part_ref.go`, with tests). Used for open, save, Save All, forwards and decrypted emails.
- Save All numbers repeated names, "file (2).html", doesn't replace existing files, and uses only the file name itself.
- A kept copy whose size doesn't match is downloaded again.
- Decrypted emails: inline files are now listed too (the earlier hidden-attachments fix didn't reach them, they were dropped in the backend), and files are opened by position. Opening one used to crash on an empty message ID.

## Fix: attachments when sending, forwarding and using the pop-out composer

- **Deleted images were still sent.** An image dropped into the text and then deleted stayed in the email invisibly. Only images still in the text are sent now. In plain text there is no HTML to check, so they all are. An inline file the text doesn't use (old drafts, some forwards) shows as a normal attachment instead.
- **Forwards left out inline files without a Content-ID**, such as Apple Mail photos. A forward now carries every file: images the body displays stay embedded, the rest are normal attachments. The same goes for editing a draft written elsewhere.
- **The pop-out composer's forwards had no attachments**, and its replies ignored Reply-To and loaded remote images, because it used a simplified copy of the reply code. It now uses the main window's (`prepareReply` in `app/original_attachments.go`, with tests); the copy is removed. This mattered since the separate-window setting also covers replies and forwards.

## Add to calendar reads the whole email

"Add to calendar" only used the first date and the top of the text. It now fills in title, start, end, location and a short summary.

- An attached calendar file is used first (`GetEmailEvents`, `app/ics_email.go`); with several events you choose one. If it can't be fetched or read, the text is used.
- Otherwise labelled details are read in English, German, French and Spanish: check-in/check-out, start/end, from/to, date, time, duration, location (`extractEvent.ts`). A text part without labels falls back to the HTML part. With no labels at all it guesses the first upcoming date, as before.
- Hotel stays are all-day, from check-in to check-out day, titled with the hotel's name; the times are in the description.
- PINs and passwords, image placeholders and (except in calendar files) links are left out of the description.
- Nothing is sent anywhere; an attached file is fetched from your own mail server.
- Also fixed: events from `.ics` files showed `\,` and `\n` instead of commas and line breaks.
- Tests: `npm run test:calendar` in `frontend` (Node's own runner, no new packages).

## New icon

Emailio has its own icon: `build/appicon.png` (1024), `build/windows/icon.ico` (16 to 256) and `frontend/src/assets/images/logo-universal.png` (Settings → About). The drawing is trimmed to its full width of 59 squares. Wails only makes `icon.ico` from `appicon.png` if it's missing.

## Closing a composer window asks about the draft

Only the composer's own title bar button asked. Closing through Windows (native title bar, Alt+F4, taskbar) closed at once and could lose the last seconds of text.

- Every way of closing the pop-out composer now asks to save, discard or keep editing (`app/composer_close.go`, with tests). While it is loading or failed to load, it closes without asking.
- Save & Close saves the latest text even if an autosave is running, and keeps the window open if saving fails.
- A draft still uploading is finished in the background after the window disappears, for up to 20 seconds. If that isn't enough it stays in the local database and is uploaded at the next start.
- When Emailio itself is closed, open composers save their draft and then close. They used to close after a second without saving.
- "Keep editing" restarts the autosave that asking had cancelled.
