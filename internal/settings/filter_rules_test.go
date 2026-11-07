package settings

import (
	"fmt"
	"testing"

	"github.com/hkdb/aerion/internal/rules"
)

func validRule(domain string) rules.Rule {
	return rules.Rule{Enabled: true, SenderMode: rules.ModeDomain, SenderValue: domain, Archive: true}
}

func ruleIDs(t *testing.T, store *Store) []string {
	t.Helper()
	list, err := store.GetFilterRules()
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, r := range list {
		ids = append(ids, r.ID)
	}
	return ids
}

func TestFilterRulesStartEmpty(t *testing.T) {
	list, err := NewStore(openTestDB(t)).GetFilterRules()
	if err != nil {
		t.Fatal(err)
	}
	if list == nil || len(list) != 0 {
		t.Errorf("got %v, want an empty non-nil list", list)
	}
}

func TestSaveFilterRule(t *testing.T) {
	store := NewStore(openTestDB(t))

	saved, err := store.SaveFilterRule(validRule("*@amazon.de"))
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID == "" {
		t.Error("expected an ID to be assigned")
	}
	if saved.SenderValue != "amazon.de" {
		t.Errorf("value = %q, want it normalized", saved.SenderValue)
	}

	saved.Enabled = false
	if _, err := store.SaveFilterRule(saved); err != nil {
		t.Fatal(err)
	}
	list, _ := store.GetFilterRules()
	if len(list) != 1 || list[0].Enabled {
		t.Errorf("got %+v, want one disabled rule", list)
	}
}

func TestSaveFilterRuleRejectsInvalid(t *testing.T) {
	store := NewStore(openTestDB(t))

	if _, err := store.SaveFilterRule(rules.Rule{Archive: true}); err == nil {
		t.Error("expected an error for a rule with no condition")
	}
	unknown := validRule("amazon.de")
	unknown.ID = "missing"
	if _, err := store.SaveFilterRule(unknown); err == nil {
		t.Error("expected an error for an unknown ID")
	}
	if list, _ := store.GetFilterRules(); len(list) != 0 {
		t.Errorf("nothing should have been saved, got %+v", list)
	}
}

func TestMoveAndDeleteFilterRule(t *testing.T) {
	store := NewStore(openTestDB(t))
	a, _ := store.SaveFilterRule(validRule("a.de"))
	b, _ := store.SaveFilterRule(validRule("b.de"))
	c, _ := store.SaveFilterRule(validRule("c.de"))

	assertOrder := func(want ...string) {
		t.Helper()
		got := ruleIDs(t, store)
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("order = %v, want %v", got, want)
		}
	}

	assertOrder(a.ID, b.ID, c.ID)

	if err := store.MoveFilterRule(c.ID, -1); err != nil {
		t.Fatal(err)
	}
	assertOrder(a.ID, c.ID, b.ID)

	if err := store.MoveFilterRule(a.ID, 1); err != nil {
		t.Fatal(err)
	}
	assertOrder(c.ID, a.ID, b.ID)

	// Past either end does nothing
	if err := store.MoveFilterRule(c.ID, -1); err != nil {
		t.Fatal(err)
	}
	if err := store.MoveFilterRule(b.ID, 1); err != nil {
		t.Fatal(err)
	}
	assertOrder(c.ID, a.ID, b.ID)

	if err := store.MoveFilterRule("missing", 1); err == nil {
		t.Error("expected an error for an unknown ID")
	}

	if err := store.DeleteFilterRule(a.ID); err != nil {
		t.Fatal(err)
	}
	assertOrder(c.ID, b.ID)
}

func TestFilterRuleMarker(t *testing.T) {
	store := NewStore(openTestDB(t))

	if _, _, ok, err := store.GetFilterRuleMarker("inbox"); err != nil || ok {
		t.Errorf("ok = %v, err = %v; want no marker", ok, err)
	}
	if err := store.SetFilterRuleMarker("inbox", 12345, 678); err != nil {
		t.Fatal(err)
	}
	validity, uid, ok, err := store.GetFilterRuleMarker("inbox")
	if err != nil || !ok || validity != 12345 || uid != 678 {
		t.Errorf("got %d, %d, %v, %v", validity, uid, ok, err)
	}
	if _, _, ok, _ := store.GetFilterRuleMarker("other"); ok {
		t.Error("markers should be per folder")
	}

	if err := store.Set(filterRuleMarkerPrefix+"bad", "not-a-marker"); err != nil {
		t.Fatal(err)
	}
	if _, _, ok, err := store.GetFilterRuleMarker("bad"); err != nil || ok {
		t.Errorf("ok = %v, err = %v; a corrupt marker should read as none", ok, err)
	}
}

func TestFiledMessageIDs(t *testing.T) {
	store := NewStore(openTestDB(t))

	filed, err := store.GetFiledMessageIDs()
	if err != nil || len(filed) != 0 {
		t.Fatalf("got %v, %v", filed, err)
	}

	if err := store.AddFiledMessageIDs([]string{"a", "b", "", "a"}); err != nil {
		t.Fatal(err)
	}
	filed, _ = store.GetFiledMessageIDs()
	if len(filed) != 2 || !filed["a"] || !filed["b"] {
		t.Errorf("got %v, want a and b only", filed)
	}
}

func TestFiledMessageIDsDropsOldest(t *testing.T) {
	store := NewStore(openTestDB(t))

	ids := make([]string, maxFiledMessageIDs+5)
	for i := range ids {
		ids[i] = fmt.Sprintf("id-%d", i)
	}
	if err := store.AddFiledMessageIDs(ids); err != nil {
		t.Fatal(err)
	}
	filed, _ := store.GetFiledMessageIDs()
	if len(filed) != maxFiledMessageIDs {
		t.Errorf("kept %d, want %d", len(filed), maxFiledMessageIDs)
	}
	if filed["id-0"] {
		t.Error("oldest should have been dropped")
	}
	if !filed[ids[len(ids)-1]] {
		t.Error("newest should be kept")
	}
}
