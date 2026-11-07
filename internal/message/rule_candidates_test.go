package message

import "testing"

func TestListRuleCandidates(t *testing.T) {
	store, accountID, folderID := newBodyFailedTestStore(t)

	insert := func(id string, uid int64, fromEmail string, isRead int) {
		t.Helper()
		if _, err := store.db.Exec(`
			INSERT INTO messages (id, account_id, folder_id, uid, message_id, subject, from_name, from_email, is_read, date)
			VALUES (?, ?, ?, ?, ?, 'Subject', 'Name', ?, ?, '2026-11-01 10:00:00 +0000 UTC')`,
			id, accountID, folderID, uid, "<"+id+"@example.com>", fromEmail, isRead); err != nil {
			t.Fatal(err)
		}
	}
	insert("a", 10, "a@example.com", 0)
	insert("b", 20, "b@example.com", 1)
	insert("temp", -5, "temp@example.com", 0)

	all, err := store.ListRuleCandidates(folderID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].ID != "a" || all[1].ID != "b" {
		t.Fatalf("got %+v, want a and b, without the temporary row", all)
	}
	if all[0].FromEmail != "a@example.com" || all[0].IsRead || !all[1].IsRead || all[0].MessageID != "<a@example.com>" || all[0].UID != 10 {
		t.Errorf("fields not read correctly: %+v", all)
	}

	newer, err := store.ListRuleCandidates(folderID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(newer) != 1 || newer[0].ID != "b" {
		t.Errorf("got %+v, want only b", newer)
	}

	maxUID, err := store.MaxUID(folderID)
	if err != nil || maxUID != 20 {
		t.Errorf("MaxUID = %d, %v; want 20", maxUID, err)
	}
}

func TestMaxUIDEmptyOrOnlyTemporary(t *testing.T) {
	store, accountID, folderID := newBodyFailedTestStore(t)

	if maxUID, err := store.MaxUID(folderID); err != nil || maxUID != 0 {
		t.Errorf("empty: MaxUID = %d, %v; want 0", maxUID, err)
	}

	if _, err := store.db.Exec(`
		INSERT INTO messages (id, account_id, folder_id, uid, subject, date)
		VALUES ('temp', ?, ?, -5, 'S', '2026-11-01 10:00:00 +0000 UTC')`, accountID, folderID); err != nil {
		t.Fatal(err)
	}
	if maxUID, err := store.MaxUID(folderID); err != nil || maxUID != 0 {
		t.Errorf("temporary only: MaxUID = %d, %v; want 0", maxUID, err)
	}
}
