package settings

import "testing"

func TestNamedSignaturesStartEmpty(t *testing.T) {
	store := NewStore(openTestDB(t))

	signatures, err := store.GetNamedSignatures()
	if err != nil {
		t.Fatal(err)
	}
	if signatures == nil || len(signatures) != 0 {
		t.Errorf("got %v, want an empty non-nil list", signatures)
	}
}

func TestSaveNamedSignature(t *testing.T) {
	store := NewStore(openTestDB(t))

	created, err := store.SaveNamedSignature(NamedSignature{Name: "  Work  ", HTML: "<p>Work</p>"})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" {
		t.Error("expected an ID to be assigned")
	}
	if created.Name != "Work" {
		t.Errorf("name = %q, want it trimmed to %q", created.Name, "Work")
	}

	created.HTML = "<p>Updated</p>"
	if _, err := store.SaveNamedSignature(created); err != nil {
		t.Fatal(err)
	}

	signatures, err := store.GetNamedSignatures()
	if err != nil {
		t.Fatal(err)
	}
	if len(signatures) != 1 || signatures[0].HTML != "<p>Updated</p>" {
		t.Errorf("got %+v, want one updated signature", signatures)
	}
}

func TestSaveNamedSignatureRejectsInvalid(t *testing.T) {
	store := NewStore(openTestDB(t))

	if _, err := store.SaveNamedSignature(NamedSignature{Name: "   "}); err == nil {
		t.Error("expected an error for a blank name")
	}
	if _, err := store.SaveNamedSignature(NamedSignature{ID: "missing", Name: "Work"}); err == nil {
		t.Error("expected an error for an unknown ID")
	}
}

func TestAccountSignatureDefault(t *testing.T) {
	store := NewStore(openTestDB(t))

	work, err := store.SaveNamedSignature(NamedSignature{Name: "Work"})
	if err != nil {
		t.Fatal(err)
	}
	short, err := store.SaveNamedSignature(NamedSignature{Name: "Short"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetAccountSignatureDefault("account-1", work.ID); err != nil {
		t.Fatal(err)
	}

	assertDefault := func(accountID, want string) {
		t.Helper()
		got, err := store.GetAccountSignatureDefault(accountID)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("default for %s = %q, want %q", accountID, got, want)
		}
	}

	assertDefault("account-1", work.ID)
	assertDefault("account-2", "")

	// Deleting another signature leaves the default alone.
	if err := store.DeleteNamedSignature(short.ID); err != nil {
		t.Fatal(err)
	}
	assertDefault("account-1", work.ID)

	// Deleting the default's signature clears it.
	if err := store.DeleteNamedSignature(work.ID); err != nil {
		t.Fatal(err)
	}
	assertDefault("account-1", "")
}
