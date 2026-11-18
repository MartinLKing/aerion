package app

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/hkdb/aerion/internal/account"
	"github.com/hkdb/aerion/internal/database"
	"github.com/hkdb/aerion/internal/draft"
	"github.com/hkdb/aerion/internal/folder"
	"github.com/hkdb/aerion/internal/message"
)

func newDraftEditTestApp(t *testing.T) (*App, *database.DB) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO accounts (id, name, email, imap_host, smtp_host, username) VALUES ('acct', 'Me', 'me@example.com', 'imap.example.com', 'smtp.example.com', 'me')`); err != nil {
		t.Fatal(err)
	}
	for _, f := range [][3]string{{"drafts", "Drafts", "drafts"}, {"inbox", "INBOX", "inbox"}} {
		if _, err := db.Exec(`INSERT INTO folders (id, account_id, name, path, folder_type) VALUES (?, 'acct', ?, ?, ?)`, f[0], f[1], f[1], f[2]); err != nil {
			t.Fatal(err)
		}
	}
	a := &App{
		accountStore:    account.NewStore(db),
		draftStore:      draft.NewStore(db),
		messageStore:    message.NewStore(db),
		folderStore:     folder.NewStore(db),
		attachmentStore: message.NewAttachmentStore(db),
	}
	a.draftOps = draftOps{accountStore: a.accountStore, draftStore: a.draftStore, messageStore: a.messageStore, folderStore: a.folderStore}
	return a, db
}

func seedDraftMessage(t *testing.T, a *App, id, folderID string, uid uint32) {
	t.Helper()
	m := &message.Message{
		ID: id, AccountID: "acct", FolderID: folderID, UID: uid,
		Subject: "Samples", FromEmail: "me@example.com", FromName: "Me",
		ToList: `[{"name":"Zafer","email":"zafer@example.com"}]`, BodyText: "Dear Zafer", BodyFetched: true,
		IsDraft: true, Date: time.Now().UTC(),
	}
	if err := a.messageStore.Create(m); err != nil {
		t.Fatal(err)
	}
}

// A draft written outside Emailio becomes a local draft that points at the
// server copy, so autosave replaces it and sending deletes it.
func TestOpenDraftForEditTakesOverServerDraft(t *testing.T) {
	a, _ := newDraftEditTestApp(t)
	seedDraftMessage(t, a, "msg-1", "drafts", 39)

	got, err := a.OpenDraftForEdit("msg-1")
	if err != nil {
		t.Fatalf("OpenDraftForEdit: %v", err)
	}
	if got.DraftID == "" || got.DraftID == "msg-1" {
		t.Fatalf("expected a new local draft ID, got %q", got.DraftID)
	}
	if got.Message.Subject != "Samples" || len(got.Message.To) != 1 || got.Message.From.Address != "me@example.com" {
		t.Errorf("unexpected content: %+v", got.Message)
	}

	d, _ := a.draftStore.Get(got.DraftID)
	if d == nil || d.IMAPUID != 39 || d.FolderID != "drafts" || !d.IsSynced() {
		t.Fatalf("local draft not linked to server copy: %+v", d)
	}

	// Opening it again finds the same draft instead of making another.
	again, err := a.OpenDraftForEdit("msg-1")
	if err != nil || again.DraftID != got.DraftID {
		t.Fatalf("second open: id=%v err=%v, want %q", again, err, got.DraftID)
	}
}

func TestOpenDraftForEditUsesExistingDraft(t *testing.T) {
	a, _ := newDraftEditTestApp(t)
	seedDraftMessage(t, a, "msg-2", "drafts", 40)
	d := &draft.Draft{AccountID: "acct", Subject: "Mine", BodyText: "x"}
	if err := a.draftStore.Create(d); err != nil {
		t.Fatal(err)
	}
	if err := a.draftStore.UpdateSyncStatus(d.ID, draft.SyncStatusSynced, 40, "drafts", ""); err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"msg-2", d.ID} {
		got, err := a.OpenDraftForEdit(id)
		if err != nil {
			t.Fatalf("OpenDraftForEdit(%s): %v", id, err)
		}
		if got.DraftID != d.ID {
			t.Errorf("OpenDraftForEdit(%s) = %q, want %q", id, got.DraftID, d.ID)
		}
	}
}

func TestOpenDraftForEditRefusesNonDraft(t *testing.T) {
	a, _ := newDraftEditTestApp(t)
	seedDraftMessage(t, a, "msg-3", "inbox", 5)
	if _, err := a.OpenDraftForEdit("msg-3"); err == nil {
		t.Fatal("a message outside Drafts must not be taken over")
	}
	if _, err := a.OpenDraftForEdit("nope"); err == nil {
		t.Fatal("unknown ID should fail")
	}
}
