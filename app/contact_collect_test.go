package app

import (
	"testing"
	"time"

	"github.com/hkdb/aerion/internal/contact"
	"github.com/hkdb/aerion/internal/message"
	"github.com/hkdb/aerion/internal/settings"
	"github.com/hkdb/aerion/internal/smtp"
)

func TestLearnContactsFromSent(t *testing.T) {
	a, db := newDraftEditTestApp(t)
	a.contactStore = contact.NewStore(db.DB)
	a.settingsStore = settings.NewStore(db)
	if _, err := db.Exec(`INSERT INTO folders (id, account_id, name, path, folder_type) VALUES ('sent','acct','Sent','Sent','sent')`); err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC().Truncate(time.Second).Add(-48 * time.Hour)
	uid := uint32(0)
	add := func(to string, at time.Time) {
		t.Helper()
		uid++
		if err := a.messageStore.Create(&message.Message{
			ID: to + at.String(), AccountID: "acct", FolderID: "sent", UID: uid, Date: at,
			ToList: `[{"name":"","email":"` + to + `"},{"name":"Me","email":"ME@example.com"}]`,
		}); err != nil {
			t.Fatal(err)
		}
	}
	add("old@x.com", base)
	add("new@x.com", base.Add(time.Hour))

	if n := a.learnContacts("acct", "sent"); n != 2 {
		t.Fatalf("added %d, want 2 (own address skipped)", n)
	}
	c, err := a.contactStore.Get("old@x.com")
	if err != nil || c == nil || !c.LastUsed.Equal(base) {
		t.Fatalf("real sent date not kept: %+v %v", c, err)
	}

	// A deleted contact isn't brought back by mail already read.
	if err := a.contactStore.Delete("old@x.com"); err != nil {
		t.Fatal(err)
	}
	add("later@x.com", base.Add(2*time.Hour))
	if n := a.learnContacts("acct", "sent"); n != 1 {
		t.Fatalf("second run added %d, want 1", n)
	}
	if c, _ := a.contactStore.Get("old@x.com"); c != nil {
		t.Fatal("deleted contact came back")
	}

	// Already known: date moves forward, count doesn't change.
	add("new@x.com", base.Add(3*time.Hour))
	if n := a.learnContacts("acct", "sent"); n != 0 {
		t.Fatalf("third run added %d, want 0", n)
	}
	c, _ = a.contactStore.Get("new@x.com")
	if c.SendCount != 1 || !c.LastUsed.Equal(base.Add(3*time.Hour)) {
		t.Fatalf("known contact: %+v", c)
	}
}

func TestCollectRecipients(t *testing.T) {
	a, db := newDraftEditTestApp(t)
	ops := &composeOps{accountStore: a.accountStore, contactStore: contact.NewStore(db.DB)}
	addr := func(e string) smtp.Address { return smtp.Address{Address: e} }
	ops.collectRecipients(&smtp.ComposeMessage{
		To:  []smtp.Address{addr("to@x.com")},
		Cc:  []smtp.Address{addr("cc@x.com"), addr("TO@x.com")},
		Bcc: []smtp.Address{addr("bcc@x.com"), addr("me@example.com")},
	})
	for _, e := range []string{"to@x.com", "cc@x.com", "bcc@x.com"} {
		if c, _ := ops.contactStore.Get(e); c == nil || c.SendCount != 1 {
			t.Errorf("%s: %+v", e, c)
		}
	}
	if c, _ := ops.contactStore.Get("me@example.com"); c != nil {
		t.Error("own address must not be saved")
	}
}
