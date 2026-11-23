package contact

import (
	"errors"
	"testing"
)

func wantState(t *testing.T, s *Store, email, state, name string) {
	t.Helper()
	got, err := s.GetSavedState(email)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != state || got.Name != name {
		t.Errorf("%s = %+v, want %s/%q", email, got, state, name)
	}
}

func TestSaveLocal(t *testing.T) {
	db := openTestDB(t)
	s := NewStore(db.DB)

	wantState(t, s, "new@x.com", SavedNone, "")
	if err := s.SaveLocal(" New@X.com ", "New Person"); err != nil {
		t.Fatal(err)
	}
	wantState(t, s, "new@x.com", SavedManual, "New Person")

	// A collected contact is promoted and keeps its history.
	if err := s.AddOrUpdate("auto@x.com", ""); err != nil {
		t.Fatal(err)
	}
	wantState(t, s, "auto@x.com", SavedCollected, "")
	if err := s.SaveLocal("auto@x.com", "Auto"); err != nil {
		t.Fatal(err)
	}
	// Later mail doesn't overwrite the chosen name.
	if err := s.AddOrUpdate("auto@x.com", "Something Else"); err != nil {
		t.Fatal(err)
	}
	wantState(t, s, "auto@x.com", SavedManual, "Auto")
	c, _ := s.Get("auto@x.com")
	if c.SendCount != 2 {
		t.Errorf("send count = %d, want 2", c.SendCount)
	}

	// Renaming doesn't make a second contact.
	if err := s.SaveLocal("auto@x.com", "Renamed"); err != nil {
		t.Fatal(err)
	}
	wantState(t, s, "auto@x.com", SavedManual, "Renamed")
	if n, _ := s.Count(); n != 2 {
		t.Errorf("count = %d, want 2", n)
	}

	if err := s.SaveLocal("nonsense", "X"); err == nil {
		t.Error("an invalid address should be refused")
	}
}

func TestSaveLocalLeavesSyncedAlone(t *testing.T) {
	db := openTestDB(t)
	s := NewStore(db.DB)
	for _, q := range []string{
		`INSERT INTO contact_sources (id, name, type, url, username, enabled) VALUES ('src','Book','carddav','https://x','u',1)`,
		`INSERT INTO contact_source_addressbooks (id, source_id, path, name, enabled) VALUES ('ab','src','/ab','AB',1)`,
		`INSERT INTO contact_records (id, source, fn) VALUES ('r1','carddav','Synced Name')`,
		`INSERT INTO contact_emails (record_id, email, is_primary) VALUES ('r1','s@x.com',1)`,
		`INSERT INTO carddav_record_state (record_id, addressbook_id, href) VALUES ('r1','ab','/r1')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	wantState(t, s, "s@x.com", SavedSynced, "Synced Name")
	if err := s.SaveLocal("s@x.com", "Other"); !errors.Is(err, ErrContactExists) {
		t.Errorf("err = %v, want ErrContactExists", err)
	}
	// With its address book switched off it can be saved separately.
	if _, err := db.Exec(`UPDATE contact_sources SET enabled = 0`); err != nil {
		t.Fatal(err)
	}
	wantState(t, s, "s@x.com", SavedNone, "")
	if err := s.SaveLocal("s@x.com", "Mine"); err != nil {
		t.Fatal(err)
	}
	wantState(t, s, "s@x.com", SavedManual, "Mine")
}
