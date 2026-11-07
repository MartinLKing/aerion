package rules

import "testing"

func TestRuleMatches(t *testing.T) {
	amazon := Candidate{FromName: "Amazon.de", FromEmail: "Versand@amazon.de", Subject: "Ihre Bestellung wurde versandt"}

	tests := []struct {
		name string
		rule Rule
		c    Candidate
		want bool
	}{
		{"domain matches", Rule{SenderMode: ModeDomain, SenderValue: "amazon.de"}, amazon, true},
		{"domain ignores case", Rule{SenderMode: ModeDomain, SenderValue: "AMAZON.DE"}, amazon, true},
		{"domain matches subdomain", Rule{SenderMode: ModeDomain, SenderValue: "amazon.de"}, Candidate{FromEmail: "x@mail.amazon.de"}, true},
		{"domain rejects lookalike", Rule{SenderMode: ModeDomain, SenderValue: "amazon.de"}, Candidate{FromEmail: "x@notamazon.de"}, false},
		{"domain rejects other tld", Rule{SenderMode: ModeDomain, SenderValue: "amazon.de"}, Candidate{FromEmail: "x@amazon.de.evil.com"}, false},
		{"domain needs an address", Rule{SenderMode: ModeDomain, SenderValue: "amazon.de"}, Candidate{FromEmail: "amazon.de"}, false},
		{"domain checks address not name", Rule{SenderMode: ModeDomain, SenderValue: "amazon.de"}, Candidate{FromName: "x@amazon.de", FromEmail: "x@other.com"}, false},

		{"sender contains email", Rule{SenderMode: ModeContains, SenderValue: "versand"}, amazon, true},
		{"sender contains name", Rule{SenderMode: ModeContains, SenderValue: "amazon.de"}, Candidate{FromName: "Amazon.de", FromEmail: "x@y.com"}, true},
		{"sender contains misses", Rule{SenderMode: ModeContains, SenderValue: "ebay"}, amazon, false},
		{"sender equals email", Rule{SenderMode: ModeEquals, SenderValue: "versand@amazon.de"}, amazon, true},
		{"sender equals name", Rule{SenderMode: ModeEquals, SenderValue: "amazon.de"}, amazon, true},
		{"sender equals needs whole value", Rule{SenderMode: ModeEquals, SenderValue: "versand"}, amazon, false},

		{"subject contains", Rule{SubjectMode: ModeContains, SubjectValue: "BESTELLUNG"}, amazon, true},
		{"subject contains misses", Rule{SubjectMode: ModeContains, SubjectValue: "rechnung"}, amazon, false},
		{"subject equals", Rule{SubjectMode: ModeEquals, SubjectValue: "ihre bestellung wurde versandt"}, amazon, true},
		{"subject equals needs whole value", Rule{SubjectMode: ModeEquals, SubjectValue: "bestellung"}, amazon, false},
		{"accented subject", Rule{SubjectMode: ModeContains, SubjectValue: "RÉPONSE"}, Candidate{Subject: "Votre réponse"}, true},

		{"both conditions met", Rule{SenderMode: ModeDomain, SenderValue: "amazon.de", SubjectMode: ModeContains, SubjectValue: "bestellung"}, amazon, true},
		{"only sender met", Rule{SenderMode: ModeDomain, SenderValue: "amazon.de", SubjectMode: ModeContains, SubjectValue: "rechnung"}, amazon, false},
		{"only subject met", Rule{SenderMode: ModeDomain, SenderValue: "ebay.de", SubjectMode: ModeContains, SubjectValue: "bestellung"}, amazon, false},

		{"no conditions never matches", Rule{}, amazon, false},
		{"empty value never matches", Rule{SenderMode: ModeContains}, amazon, false},
		{"empty subject value never matches", Rule{SubjectMode: ModeContains}, amazon, false},
		{"empty equals never matches empty field", Rule{SubjectMode: ModeEquals}, Candidate{}, false},
		{"unknown mode never matches", Rule{SenderMode: "regex", SenderValue: "amazon"}, amazon, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rule.Matches(tt.c); got != tt.want {
				t.Errorf("Matches = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeDomain(t *testing.T) {
	for input, want := range map[string]string{
		"amazon.de":           "amazon.de",
		"  amazon.de ":        "amazon.de",
		"*@amazon.de":         "amazon.de",
		"@amazon.de":          "amazon.de",
		"*.amazon.de":         "amazon.de",
		"someone@amazon.de":   "amazon.de",
		"*someone@amazon.de ": "amazon.de",
	} {
		got := Rule{SenderMode: ModeDomain, SenderValue: input}.Normalize().SenderValue
		if got != want {
			t.Errorf("Normalize(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeKeepsOtherModes(t *testing.T) {
	got := Rule{SenderMode: ModeContains, SenderValue: " @amazon ", SubjectValue: " sale "}.Normalize()
	if got.SenderValue != "@amazon" || got.SubjectValue != "sale" {
		t.Errorf("got %+v", got)
	}
}

func TestValidate(t *testing.T) {
	valid := Rule{SenderMode: ModeDomain, SenderValue: "amazon.de", Archive: true}

	tests := []struct {
		name    string
		rule    Rule
		wantErr bool
	}{
		{"archive by domain", valid, false},
		{"mark read by subject", Rule{SubjectMode: ModeContains, SubjectValue: "x", MarkRead: true}, false},
		{"archive and mark read", Rule{SubjectMode: ModeEquals, SubjectValue: "x", Archive: true, MarkRead: true}, false},
		{"move with account", Rule{SubjectMode: ModeEquals, SubjectValue: "x", AccountID: "a", MoveToFolderID: "f"}, false},
		{"no condition", Rule{Archive: true}, true},
		{"blank sender value", Rule{SenderMode: ModeContains, Archive: true}, true},
		{"blank subject value", Rule{SubjectMode: ModeContains, Archive: true}, true},
		{"domain without dot", Rule{SenderMode: ModeDomain, SenderValue: "amazon", Archive: true}, true},
		{"no action", Rule{SubjectMode: ModeEquals, SubjectValue: "x"}, true},
		{"archive and move", Rule{SubjectMode: ModeEquals, SubjectValue: "x", AccountID: "a", Archive: true, MoveToFolderID: "f"}, true},
		{"move without account", Rule{SubjectMode: ModeEquals, SubjectValue: "x", MoveToFolderID: "f"}, true},
		{"unknown sender mode", Rule{SenderMode: "regex", SenderValue: "x", Archive: true}, true},
		{"domain mode on subject", Rule{SubjectMode: ModeDomain, SubjectValue: "a.de", Archive: true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.rule.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSelect(t *testing.T) {
	amazonRule := Rule{ID: "amazon", Enabled: true, SenderMode: ModeDomain, SenderValue: "amazon.de", Archive: true}
	saleRule := Rule{ID: "sale", Enabled: true, SubjectMode: ModeContains, SubjectValue: "sale", MarkRead: true}

	amazonSale := Candidate{ID: "1", MessageID: "m1", FromEmail: "x@amazon.de", Subject: "Big sale"}
	otherSale := Candidate{ID: "2", MessageID: "m2", FromEmail: "x@other.com", Subject: "Big sale"}
	personal := Candidate{ID: "3", MessageID: "m3", FromEmail: "friend@other.com", Subject: "Hello"}

	ids := func(g Group) []string {
		out := []string{}
		for _, c := range g.Messages {
			out = append(out, c.ID)
		}
		return out
	}

	t.Run("first matching rule wins", func(t *testing.T) {
		groups := Select([]Rule{amazonRule, saleRule}, "a", []Candidate{amazonSale, otherSale, personal}, nil)
		if len(groups) != 2 {
			t.Fatalf("got %d groups, want 2", len(groups))
		}
		if groups[0].Rule.ID != "amazon" || len(ids(groups[0])) != 1 || ids(groups[0])[0] != "1" {
			t.Errorf("amazon group = %v", ids(groups[0]))
		}
		if groups[1].Rule.ID != "sale" || len(ids(groups[1])) != 1 || ids(groups[1])[0] != "2" {
			t.Errorf("sale group = %v", ids(groups[1]))
		}
	})

	t.Run("order decides which rule wins", func(t *testing.T) {
		groups := Select([]Rule{saleRule, amazonRule}, "a", []Candidate{amazonSale}, nil)
		if len(groups) != 1 || groups[0].Rule.ID != "sale" {
			t.Errorf("got %+v, want the sale rule", groups)
		}
	})

	t.Run("disabled rule is skipped", func(t *testing.T) {
		off := amazonRule
		off.Enabled = false
		if groups := Select([]Rule{off}, "a", []Candidate{amazonSale}, nil); len(groups) != 0 {
			t.Errorf("got %d groups, want none", len(groups))
		}
	})

	t.Run("rule for another account is skipped", func(t *testing.T) {
		scoped := amazonRule
		scoped.AccountID = "b"
		if groups := Select([]Rule{scoped}, "a", []Candidate{amazonSale}, nil); len(groups) != 0 {
			t.Errorf("got %d groups, want none", len(groups))
		}
		if groups := Select([]Rule{scoped}, "b", []Candidate{amazonSale}, nil); len(groups) != 1 {
			t.Errorf("got %d groups, want 1 for its own account", len(groups))
		}
	})

	t.Run("message moved back is not filed again", func(t *testing.T) {
		groups := Select([]Rule{amazonRule}, "a", []Candidate{amazonSale}, map[string]bool{"m1": true})
		if len(groups) != 0 {
			t.Errorf("got %d groups, want none", len(groups))
		}
	})

	t.Run("message without a Message-ID still matches", func(t *testing.T) {
		noID := amazonSale
		noID.MessageID = ""
		groups := Select([]Rule{amazonRule}, "a", []Candidate{noID}, map[string]bool{"": true})
		if len(groups) != 1 {
			t.Errorf("got %d groups, want 1", len(groups))
		}
	})

	t.Run("no rules", func(t *testing.T) {
		if groups := Select(nil, "a", []Candidate{amazonSale}, nil); len(groups) != 0 {
			t.Errorf("got %d groups, want none", len(groups))
		}
	})
}

func TestMatchAllRunsDisabledRule(t *testing.T) {
	r := Rule{Enabled: false, SenderMode: ModeDomain, SenderValue: "amazon.de", Archive: true}
	matches := MatchAll(r, []Candidate{{ID: "1", FromEmail: "x@amazon.de"}, {ID: "2", FromEmail: "x@other.com"}})
	if len(matches) != 1 || matches[0].ID != "1" {
		t.Errorf("got %+v", matches)
	}
}
