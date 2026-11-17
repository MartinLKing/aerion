package draft

import (
	"path/filepath"
	"testing"

	"github.com/hkdb/aerion/internal/database"
)

func openTestDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func insertTestAccount(t *testing.T, db *database.DB) string {
	t.Helper()
	id := "test-account-1"
	_, err := db.Exec(`INSERT INTO accounts (id, name, email, imap_host, imap_port, smtp_host, smtp_port, auth_type, username)
		VALUES (?, 'Test', 'test@example.com', 'imap.example.com', 993, 'smtp.example.com', 587, 'password', 'test')`, id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestCreateAndGet(t *testing.T) {
	db := openTestDB(t)
	accountID := insertTestAccount(t, db)
	store := NewStore(db)

	d := &Draft{
		AccountID: accountID,
		ToList:    `["alice@example.com"]`,
		Subject:   "Test Draft",
		BodyHTML:  "<p>Hello</p>",
		BodyText:  "Hello",
	}

	if err := store.Create(d); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.ID == "" {
		t.Fatal("expected ID to be set after Create")
	}
	if d.SyncStatus != SyncStatusPending {
		t.Errorf("SyncStatus = %q, want %q", d.SyncStatus, SyncStatusPending)
	}

	got, err := store.Get(d.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected draft, got nil")
	}
	if got.Subject != "Test Draft" {
		t.Errorf("Subject = %q, want %q", got.Subject, "Test Draft")
	}
	if got.AccountID != accountID {
		t.Errorf("AccountID = %q, want %q", got.AccountID, accountID)
	}
	if got.ToList != `["alice@example.com"]` {
		t.Errorf("ToList = %q, want %q", got.ToList, `["alice@example.com"]`)
	}
}

func TestListByAccount(t *testing.T) {
	db := openTestDB(t)
	accountID := insertTestAccount(t, db)
	store := NewStore(db)

	drafts := []*Draft{
		{AccountID: accountID, Subject: "Draft 1", BodyText: "one"},
		{AccountID: accountID, Subject: "Draft 2", BodyText: "two"},
	}
	for _, d := range drafts {
		if err := store.Create(d); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	list, err := store.ListByAccount(accountID)
	if err != nil {
		t.Fatalf("ListByAccount failed: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("ListByAccount returned %d drafts, want 2", len(list))
	}
}

func TestUpdate(t *testing.T) {
	db := openTestDB(t)
	accountID := insertTestAccount(t, db)
	store := NewStore(db)

	d := &Draft{
		AccountID: accountID,
		Subject:   "Original",
		BodyText:  "original",
	}
	if err := store.Create(d); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	d.Subject = "Updated"
	if err := store.Update(d); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	got, err := store.Get(d.ID)
	if err != nil {
		t.Fatalf("Get after update failed: %v", err)
	}
	if got.Subject != "Updated" {
		t.Errorf("Subject = %q, want %q", got.Subject, "Updated")
	}
}

func TestDelete(t *testing.T) {
	db := openTestDB(t)
	accountID := insertTestAccount(t, db)
	store := NewStore(db)

	d := &Draft{
		AccountID: accountID,
		Subject:   "ToDelete",
		BodyText:  "delete me",
	}
	if err := store.Create(d); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := store.Delete(d.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	got, err := store.Get(d.ID)
	if err != nil {
		t.Fatalf("Get after delete failed: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil after delete, got %+v", got)
	}
}

func insertTestFolder(t *testing.T, db *database.DB, accountID string) string {
	t.Helper()
	folderID := "folder-123"
	_, err := db.Exec(`INSERT INTO folders (id, account_id, name, path, folder_type) VALUES (?, ?, 'Drafts', 'Drafts', 'drafts')`, folderID, accountID)
	if err != nil {
		t.Fatal(err)
	}
	return folderID
}

func TestSyncStatusUpdate(t *testing.T) {
	db := openTestDB(t)
	accountID := insertTestAccount(t, db)
	folderID := insertTestFolder(t, db, accountID)
	store := NewStore(db)

	d := &Draft{
		AccountID: accountID,
		Subject:   "Sync Test",
		BodyText:  "sync",
	}
	if err := store.Create(d); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.SyncStatus != SyncStatusPending {
		t.Fatalf("initial SyncStatus = %q, want %q", d.SyncStatus, SyncStatusPending)
	}

	var uid uint32 = 42
	if err := store.UpdateSyncStatus(d.ID, SyncStatusSynced, uid, folderID, ""); err != nil {
		t.Fatalf("UpdateSyncStatus failed: %v", err)
	}

	got, err := store.Get(d.ID)
	if err != nil {
		t.Fatalf("Get after sync update failed: %v", err)
	}
	if got.SyncStatus != SyncStatusSynced {
		t.Errorf("SyncStatus = %q, want %q", got.SyncStatus, SyncStatusSynced)
	}
	if got.IMAPUID != 42 {
		t.Errorf("IMAPUID = %d, want 42", got.IMAPUID)
	}
	if got.FolderID != folderID {
		t.Errorf("FolderID = %q, want %q", got.FolderID, folderID)
	}
}

// A content save made with a copy of the draft read before the IMAP sync
// finished must not wipe the server UID the sync recorded meanwhile.
func TestUpdateKeepsServerCopy(t *testing.T) {
	db := openTestDB(t)
	accountID := insertTestAccount(t, db)
	folderID := insertTestFolder(t, db, accountID)
	store := NewStore(db)

	d := &Draft{AccountID: accountID, Subject: "v1", BodyText: "v1"}
	if err := store.Create(d); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	stale, _ := store.Get(d.ID) // read before the sync recorded the UID

	if err := store.UpdateSyncStatus(d.ID, SyncStatusSynced, 39, folderID, ""); err != nil {
		t.Fatalf("UpdateSyncStatus failed: %v", err)
	}

	stale.Subject = "v2"
	stale.SyncStatus = SyncStatusPending
	if err := store.Update(stale); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	got, _ := store.Get(d.ID)
	if got.Subject != "v2" {
		t.Errorf("Subject = %q, want v2", got.Subject)
	}
	if got.IMAPUID != 39 || got.FolderID != folderID {
		t.Errorf("server copy lost: uid=%d folder=%q, want 39/%q", got.IMAPUID, got.FolderID, folderID)
	}
}

func TestMarkSyncFailedKeepsServerCopy(t *testing.T) {
	db := openTestDB(t)
	accountID := insertTestAccount(t, db)
	folderID := insertTestFolder(t, db, accountID)
	store := NewStore(db)

	d := &Draft{AccountID: accountID, Subject: "s", BodyText: "s"}
	if err := store.Create(d); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if err := store.UpdateSyncStatus(d.ID, SyncStatusSynced, 7, folderID, ""); err != nil {
		t.Fatalf("UpdateSyncStatus failed: %v", err)
	}
	if err := store.MarkSyncFailed(d.ID, "connection lost"); err != nil {
		t.Fatalf("MarkSyncFailed failed: %v", err)
	}

	got, _ := store.Get(d.ID)
	if got.SyncStatus != SyncStatusFailed || got.SyncError != "connection lost" {
		t.Errorf("status=%q error=%q", got.SyncStatus, got.SyncError)
	}
	if got.IMAPUID != 7 || got.FolderID != folderID {
		t.Errorf("server copy lost: uid=%d folder=%q", got.IMAPUID, got.FolderID)
	}
}
