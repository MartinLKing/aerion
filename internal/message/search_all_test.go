package message

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hkdb/aerion/internal/database"
)

type searchFixture struct {
	t     *testing.T
	store *Store
	db    *database.DB
	next  int
}

func newSearchFixture(t *testing.T) *searchFixture {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}

	f := &searchFixture{t: t, store: NewStore(db), db: db}
	for _, id := range []string{"gmail", "outlook"} {
		f.exec(`INSERT INTO accounts (id, name, email, imap_host, smtp_host, username) VALUES (?, ?, ?, 'h', 'h', ?)`,
			id, id, id+"@example.com", id)
		for _, folderType := range []string{"inbox", "sent", "drafts", "trash", "spam", "folder"} {
			f.exec(`INSERT INTO folders (id, account_id, name, path, folder_type) VALUES (?, ?, ?, ?, ?)`,
				id+"-"+folderType, id, folderType, folderType, folderType)
		}
	}
	return f
}

func (f *searchFixture) exec(query string, args ...any) sql.Result {
	f.t.Helper()
	result, err := f.db.Exec(query, args...)
	if err != nil {
		f.t.Fatalf("exec: %v", err)
	}
	return result
}

// addMessage inserts a message into "<account>-<folderType>".
func (f *searchFixture) addMessage(account, folderType, threadID, subject, body string) string {
	f.t.Helper()
	f.next++
	id := fmt.Sprintf("msg-%d", f.next)
	f.exec(`INSERT INTO messages (id, account_id, folder_id, uid, thread_id, subject, from_name, from_email, to_list, body_text, date)
	        VALUES (?, ?, ?, ?, ?, ?, 'Ann', 'ann@sender.example', '[]', ?, '2026-11-01 10:00:00 +0000 UTC')`,
		id, account, account+"-"+folderType, f.next, threadID, subject, body)
	return id
}

func (f *searchFixture) search(query string) []*ConversationSearchResult {
	f.t.Helper()
	results, count, err := f.store.SearchConversationsAllMail(query, 0, 50, "")
	if err != nil {
		f.t.Fatal(err)
	}
	if count != len(results) {
		f.t.Errorf("count = %d but got %d results", count, len(results))
	}
	return results
}

func TestSearchAllMailCoversAccountsAndFolders(t *testing.T) {
	f := newSearchFixture(t)
	f.addMessage("gmail", "inbox", "t1", "invoice one", "")
	f.addMessage("gmail", "sent", "t2", "invoice two", "")
	f.addMessage("outlook", "inbox", "t3", "invoice three", "")
	f.addMessage("outlook", "folder", "t4", "invoice four", "")
	f.addMessage("gmail", "inbox", "t5", "unrelated", "")

	results := f.search("invoice")
	if len(results) != 4 {
		t.Fatalf("got %d results, want 4", len(results))
	}
	accounts := map[string]bool{}
	for _, r := range results {
		accounts[r.AccountID] = true
		if r.AccountName == "" || r.FolderName == "" {
			t.Errorf("result missing account or folder name: %+v", r)
		}
	}
	if !accounts["gmail"] || !accounts["outlook"] {
		t.Errorf("expected results from both accounts, got %v", accounts)
	}
}

func TestSearchAllMailSkipsTrashAndSpam(t *testing.T) {
	f := newSearchFixture(t)
	f.addMessage("gmail", "trash", "t1", "invoice deleted", "")
	f.addMessage("gmail", "spam", "t2", "invoice junk", "")

	if results := f.search("invoice"); len(results) != 0 {
		t.Errorf("got %d results, want none", len(results))
	}
}

func TestSearchAllMailMatchesBody(t *testing.T) {
	f := newSearchFixture(t)
	f.addMessage("gmail", "inbox", "t1", "hello", "the quarterly figures")

	if results := f.search("quarterly"); len(results) != 1 {
		t.Errorf("got %d results, want 1", len(results))
	}
}

func TestSearchAllMailOpensThreadFromBestFolder(t *testing.T) {
	tests := []struct {
		name  string
		setup func(f *searchFixture)
		want  string
	}{
		{
			// The match is in Sent, but opening from Sent would hide the inbox message.
			name: "match only in sent still opens from inbox",
			setup: func(f *searchFixture) {
				f.addMessage("gmail", "inbox", "t1", "hello", "")
				f.addMessage("gmail", "sent", "t1", "invoice", "")
			},
			want: "gmail-inbox",
		},
		{
			name: "other folder preferred over sent",
			setup: func(f *searchFixture) {
				f.addMessage("gmail", "folder", "t1", "hello", "")
				f.addMessage("gmail", "sent", "t1", "invoice", "")
			},
			want: "gmail-folder",
		},
		{
			name: "sent-only thread opens from sent",
			setup: func(f *searchFixture) {
				f.addMessage("gmail", "sent", "t1", "invoice", "")
			},
			want: "gmail-sent",
		},
		{
			name: "trash copy is never chosen",
			setup: func(f *searchFixture) {
				f.addMessage("gmail", "trash", "t1", "hello", "")
				f.addMessage("gmail", "sent", "t1", "invoice", "")
			},
			want: "gmail-sent",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newSearchFixture(t)
			tt.setup(f)
			results := f.search("invoice")
			if len(results) != 1 {
				t.Fatalf("got %d results, want 1", len(results))
			}
			if results[0].FolderID != tt.want {
				t.Errorf("folder = %q, want %q", results[0].FolderID, tt.want)
			}
		})
	}
}

func TestSearchAllMailKeepsSameThreadIDSeparatePerAccount(t *testing.T) {
	f := newSearchFixture(t)
	f.addMessage("gmail", "inbox", "shared", "invoice", "")
	f.addMessage("outlook", "inbox", "shared", "invoice", "")

	if results := f.search("invoice"); len(results) != 2 {
		t.Errorf("got %d results, want 2", len(results))
	}
}

func TestSearchAllMailPaginates(t *testing.T) {
	f := newSearchFixture(t)
	for i := 0; i < 5; i++ {
		f.addMessage("gmail", "inbox", fmt.Sprintf("t%d", i), "invoice", "")
	}

	results, count, err := f.store.SearchConversationsAllMail("invoice", 2, 2, "")
	if err != nil {
		t.Fatal(err)
	}
	if count != 5 || len(results) != 2 {
		t.Errorf("count = %d, results = %d; want 5 and 2", count, len(results))
	}
}

func TestSearchAllMailBlankQuery(t *testing.T) {
	f := newSearchFixture(t)
	if results, count, err := f.store.SearchConversationsAllMail("   ", 0, 50, ""); err != nil || count != 0 || results != nil {
		t.Errorf("got %v, %d, %v; want nothing", results, count, err)
	}
}

func TestBackfillBodyTextFromHTML(t *testing.T) {
	f := newSearchFixture(t)
	htmlOnly := f.addMessage("gmail", "inbox", "t1", "hello", "")
	f.exec(`UPDATE messages SET body_html = '<p>zebra crossing</p>' WHERE id = ?`, htmlOnly)

	hasText := f.addMessage("gmail", "inbox", "t2", "hello", "original text")
	f.exec(`UPDATE messages SET body_html = '<p>giraffe</p>' WHERE id = ?`, hasText)

	encrypted := f.addMessage("gmail", "inbox", "t3", "hello", "")
	f.exec(`UPDATE messages SET body_html = '<p>walrus</p>', pgp_encrypted = 1 WHERE id = ?`, encrypted)

	draft := f.addMessage("gmail", "drafts", "t4", "hello", "")
	f.exec(`UPDATE messages SET body_html = '<p>penguin</p>' WHERE id = ?`, draft)

	blank := f.addMessage("gmail", "inbox", "t5", "hello", "")
	f.exec(`UPDATE messages SET body_html = '<img src="x">' WHERE id = ?`, blank)

	stripTags := func(html string) string {
		if !strings.Contains(html, "<p>") {
			return ""
		}
		return strings.NewReplacer("<p>", "", "</p>", "").Replace(html)
	}

	if results := f.search("zebra"); len(results) != 0 {
		t.Fatalf("zebra should not match before the backfill, got %d", len(results))
	}

	updated, err := f.store.BackfillBodyTextFromHTML(context.Background(), stripTags)
	if err != nil {
		t.Fatal(err)
	}
	if updated != 1 {
		t.Errorf("updated = %d, want 1", updated)
	}

	if results := f.search("zebra"); len(results) != 1 {
		t.Errorf("zebra: got %d results, want 1 once backfilled", len(results))
	}
	for _, word := range []string{"giraffe", "walrus", "penguin"} {
		if results := f.search(word); len(results) != 0 {
			t.Errorf("%s: got %d results, want none", word, len(results))
		}
	}

	var text string
	if err := f.db.QueryRow(`SELECT body_text FROM messages WHERE id = ?`, hasText).Scan(&text); err != nil {
		t.Fatal(err)
	}
	if text != "original text" {
		t.Errorf("existing body text changed to %q", text)
	}
}
