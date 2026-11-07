// Package rules holds the filter rule definitions and matching logic. It has no
// dependencies on the rest of the app, so all of its behaviour can be unit tested.
package rules

import (
	"errors"
	"strings"
)

// How a rule compares a sender or subject. ModeAny means the field isn't checked.
const (
	ModeAny      = ""
	ModeContains = "contains"
	ModeEquals   = "equals"
	ModeDomain   = "domain" // sender only
)

// Rule files incoming Inbox mail based on its sender and/or subject.
type Rule struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
	// AccountID limits the rule to one account. Empty means all accounts.
	AccountID string `json:"accountId"`

	SenderMode   string `json:"senderMode"`
	SenderValue  string `json:"senderValue"`
	SubjectMode  string `json:"subjectMode"`
	SubjectValue string `json:"subjectValue"`

	Archive bool `json:"archive"`
	// MoveToFolderID moves matches to this folder, which must belong to AccountID.
	MoveToFolderID string `json:"moveToFolderId"`
	MarkRead       bool   `json:"markRead"`
}

// Candidate is the part of a message that rules look at or act on.
type Candidate struct {
	ID        string
	MessageID string // RFC 822 Message-ID, stable across moves unlike ID and UID
	UID       int64
	FromName  string
	FromEmail string
	Subject   string
	IsRead    bool
}

// Normalize trims the rule's values and reduces sender domain patterns such as
// "*@amazon.de" and "@amazon.de" to "amazon.de".
func (r Rule) Normalize() Rule {
	r.SenderValue = strings.TrimSpace(r.SenderValue)
	r.SubjectValue = strings.TrimSpace(r.SubjectValue)
	if r.SenderMode == ModeDomain {
		r.SenderValue = strings.TrimLeft(r.SenderValue, "*@.")
		if i := strings.LastIndex(r.SenderValue, "@"); i >= 0 {
			r.SenderValue = r.SenderValue[i+1:]
		}
	}
	return r
}

// Validate reports why a normalized rule can't be saved.
func (r Rule) Validate() error {
	switch r.SenderMode {
	case ModeAny, ModeContains, ModeEquals, ModeDomain:
	default:
		return errors.New("unknown sender condition")
	}
	switch r.SubjectMode {
	case ModeAny, ModeContains, ModeEquals:
	default:
		return errors.New("unknown subject condition")
	}

	if r.SenderMode == ModeAny && r.SubjectMode == ModeAny {
		return errors.New("a rule needs a sender or subject condition")
	}
	if r.SenderMode != ModeAny && r.SenderValue == "" {
		return errors.New("enter a value for the sender condition")
	}
	if r.SubjectMode != ModeAny && r.SubjectValue == "" {
		return errors.New("enter a value for the subject condition")
	}
	if r.SenderMode == ModeDomain && !strings.Contains(r.SenderValue, ".") {
		return errors.New("enter a domain such as amazon.de")
	}

	if !r.Archive && r.MoveToFolderID == "" && !r.MarkRead {
		return errors.New("a rule needs at least one action")
	}
	if r.Archive && r.MoveToFolderID != "" {
		return errors.New("a rule can't both archive and move to a folder")
	}
	if r.MoveToFolderID != "" && r.AccountID == "" {
		return errors.New("moving to a folder needs a specific account")
	}
	return nil
}

// Matches reports whether a message meets the rule's conditions. A rule with no
// conditions never matches, so a bad rule can't file all of the user's mail.
func (r Rule) Matches(c Candidate) bool {
	if r.SenderMode == ModeAny && r.SubjectMode == ModeAny {
		return false
	}
	return r.senderMatches(c) && r.subjectMatches(c)
}

func (r Rule) senderMatches(c Candidate) bool {
	value := strings.ToLower(r.SenderValue)
	name := strings.ToLower(strings.TrimSpace(c.FromName))
	email := strings.ToLower(strings.TrimSpace(c.FromEmail))

	switch r.SenderMode {
	case ModeAny:
		return true
	case ModeContains:
		return value != "" && (strings.Contains(email, value) || strings.Contains(name, value))
	case ModeEquals:
		return value != "" && (email == value || name == value)
	case ModeDomain:
		at := strings.LastIndex(email, "@")
		if value == "" || at < 0 {
			return false
		}
		domain := email[at+1:]
		// The dot stops "amazon.de" from matching "notamazon.de"
		return domain == value || strings.HasSuffix(domain, "."+value)
	}
	return false
}

func (r Rule) subjectMatches(c Candidate) bool {
	value := strings.ToLower(r.SubjectValue)
	subject := strings.ToLower(strings.TrimSpace(c.Subject))

	switch r.SubjectMode {
	case ModeAny:
		return true
	case ModeContains:
		return value != "" && strings.Contains(subject, value)
	case ModeEquals:
		return value != "" && subject == value
	}
	return false
}

// AppliesToAccount reports whether the rule covers the given account.
func (r Rule) AppliesToAccount(accountID string) bool {
	return r.AccountID == "" || r.AccountID == accountID
}

// Group is a rule together with the messages it should act on.
type Group struct {
	Rule     Rule
	Messages []Candidate
}

// Select assigns new messages to rules. Rules are checked in order and the first
// match wins. Disabled rules and rules for other accounts are skipped, as are
// messages in alreadyFiled, keyed by Message-ID: a message the user moved back to
// the Inbox arrives with a new UID and must not be filed again.
func Select(ruleList []Rule, accountID string, candidates []Candidate, alreadyFiled map[string]bool) []Group {
	groups := make([]Group, len(ruleList))
	for i, r := range ruleList {
		groups[i].Rule = r
	}

	for _, c := range candidates {
		if c.MessageID != "" && alreadyFiled[c.MessageID] {
			continue
		}
		for i, r := range ruleList {
			if r.Enabled && r.AppliesToAccount(accountID) && r.Matches(c) {
				groups[i].Messages = append(groups[i].Messages, c)
				break
			}
		}
	}

	matched := make([]Group, 0, len(groups))
	for _, g := range groups {
		if len(g.Messages) > 0 {
			matched = append(matched, g)
		}
	}
	return matched
}

// MatchAll returns every candidate meeting a single rule, for running it on
// existing mail. The user chose this rule, so it runs even if switched off, and
// messages it filed before can be filed again.
func MatchAll(r Rule, candidates []Candidate) []Candidate {
	var matches []Candidate
	for _, c := range candidates {
		if r.Matches(c) {
			matches = append(matches, c)
		}
	}
	return matches
}
