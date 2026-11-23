package contact

import (
	"testing"
	"time"
)

func TestSearchMostRecentFirst(t *testing.T) {
	db := openTestDB(t)
	s := NewStore(db.DB)
	for i := 0; i < 3; i++ {
		if err := s.AddOrUpdate("often@x.com", ""); err != nil {
			t.Fatal(err)
		}
	}
	// Stored in different zones: text order would be wrong, time order isn't.
	base := time.Now().Add(-time.Hour)
	if _, err := db.Exec(`UPDATE contact_emails SET last_used = ? WHERE email = 'often@x.com'`,
		base.In(time.FixedZone("JST", 9*3600))); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordSent("recent@x.com", "", base.Add(time.Minute).In(time.FixedZone("", 2*3600))); err != nil {
		t.Fatal(err)
	}
	for _, e := range []string{"b-never@x.com", "a-never@x.com"} {
		if err := s.Create(e, ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`UPDATE contact_emails SET last_used = NULL, send_count = 0 WHERE email LIKE '%never%'`); err != nil {
		t.Fatal(err)
	}

	got, err := s.Search("x.com", 10)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"recent@x.com", "often@x.com", "a-never@x.com", "b-never@x.com"}
	if len(got) != len(want) {
		t.Fatalf("got %d results", len(got))
	}
	for i := range want {
		if got[i].Email != want[i] {
			t.Errorf("position %d = %s, want %s", i, got[i].Email, want[i])
		}
	}
	if top, _ := s.Search("x.com", 1); len(top) != 1 || top[0].Email != "recent@x.com" {
		t.Errorf("limit cut off the most recent: %+v", top)
	}
}

func TestMergeKeepsNameOfDuplicate(t *testing.T) {
	now := time.Now()
	got := MergeResults([]*Contact{
		{Email: "a@x.com", Source: "aerion", LastUsed: now},
		{Email: "A@x.com", Source: "carddav", DisplayName: "Anna"},
	})
	if len(got) != 1 || got[0].DisplayName != "Anna" || !got[0].LastUsed.Equal(now) {
		t.Fatalf("merged = %+v", got)
	}
}
