package contact

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// RecordSent notes that mail was sent to an address at a given time, as learned
// from the Sent folder (Emailio). A new address becomes a collected contact. A
// known one only has its last-used time moved forward: its name and send count
// stay, since mail sent from Emailio was already counted by AddOrUpdate.
// Returns whether a contact was added.
func (s *Store) RecordSent(email, displayName string, at time.Time) (bool, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	displayName = strings.TrimSpace(displayName)
	// UTC: a time with an unnamed zone is stored as text that cannot be read back.
	at = at.UTC()
	if strings.EqualFold(displayName, email) {
		displayName = ""
	}
	if !strings.Contains(email, "@") {
		return false, nil
	}

	existing, err := s.Get(email)
	if err != nil {
		return false, err
	}
	if existing != nil {
		if !at.After(existing.LastUsed) {
			return false, nil
		}
		if _, err := s.db.Exec(`UPDATE contact_emails SET last_used = ? WHERE email = ?`, at, email); err != nil {
			return false, fmt.Errorf("failed to update last used: %w", err)
		}
		return false, nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	id := uuid.New().String()
	now := time.Now()
	if _, err := tx.Exec(`
		INSERT INTO contact_records (id, source, kind, fn, created_at, updated_at)
		VALUES (?, 'local', 'collected', ?, ?, ?)
	`, id, displayName, now, now); err != nil {
		return false, fmt.Errorf("failed to insert contact_records: %w", err)
	}
	if _, err := tx.Exec(`
		INSERT INTO contact_emails (record_id, email, send_count, last_used, name_overridden, is_primary)
		VALUES (?, ?, 1, ?, 0, 1)
	`, id, email, at); err != nil {
		return false, fmt.Errorf("failed to insert contact_emails: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}
