package contact

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Saving a contact from an email (Emailio addition).

// States of an address, from the point of view of the save button.
const (
	SavedNone      = "none"      // not a contact
	SavedCollected = "collected" // added automatically from sent mail
	SavedManual    = "saved"     // saved by the user, can be renamed
	SavedSynced    = "synced"    // in a synced address book, not editable here
	SavedOwn       = "own"       // one of the user's own addresses
)

// SavedState describes whether an address is a contact.
type SavedState struct {
	State string `json:"state"`
	Name  string `json:"name"`
}

// GetSavedState looks an address up. A contact the user saved wins over one
// from a synced address book, which wins over an automatically added one.
// Disabled address books don't count, as in Search.
func (s *Store) GetSavedState(email string) (SavedState, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	rows, err := s.db.Query(`
		SELECT cr.source, COALESCE(cr.kind, ''), COALESCE(cr.fn, '')
		FROM contact_emails ce
		JOIN contact_records cr ON cr.id = ce.record_id
		WHERE ce.email = ? AND (
			cr.source = 'local'
			OR EXISTS (
				SELECT 1 FROM carddav_record_state crs
				JOIN contact_source_addressbooks ab ON ab.id = crs.addressbook_id
				JOIN contact_sources src ON src.id = ab.source_id
				WHERE crs.record_id = cr.id AND src.enabled = 1 AND ab.enabled = 1
			)
		)
	`, email)
	if err != nil {
		return SavedState{}, fmt.Errorf("failed to look up contact: %w", err)
	}
	defer rows.Close()

	rank := map[string]int{SavedNone: 0, SavedCollected: 1, SavedSynced: 2, SavedManual: 3}
	best := SavedState{State: SavedNone}
	for rows.Next() {
		var source, kind, name string
		if err := rows.Scan(&source, &kind, &name); err != nil {
			return SavedState{}, fmt.Errorf("failed to scan contact: %w", err)
		}
		state := SavedSynced
		if source == "local" {
			state = SavedManual
			if kind == "collected" {
				state = SavedCollected
			}
		}
		if rank[state] > rank[best.State] {
			best = SavedState{State: state, Name: name}
		}
	}
	return best, rows.Err()
}

// SaveLocal saves an address as a contact the user named. An automatically
// added contact becomes a regular one; a saved one is renamed. Either way the
// name is protected from being overwritten by later mail. Synced contacts are
// never changed.
func (s *Store) SaveLocal(email, name string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	name = strings.TrimSpace(name)
	if !strings.Contains(email, "@") {
		return fmt.Errorf("not an email address: %q", email)
	}

	state, err := s.GetSavedState(email)
	if err != nil {
		return err
	}
	if state.State == SavedSynced {
		return ErrContactExists
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now()

	var id string
	err = tx.QueryRow(`
		SELECT cr.id FROM contact_records cr
		JOIN contact_emails ce ON ce.record_id = cr.id
		WHERE ce.email = ? AND cr.source = 'local'
		ORDER BY COALESCE(cr.kind, '') = 'collected' LIMIT 1
	`, email).Scan(&id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		id = uuid.New().String()
		if _, err := tx.Exec(`
			INSERT INTO contact_records (id, source, kind, fn, created_at, updated_at)
			VALUES (?, 'local', 'manual', ?, ?, ?)
		`, id, name, now, now); err != nil {
			return fmt.Errorf("failed to save contact: %w", err)
		}
		if _, err := tx.Exec(`
			INSERT INTO contact_emails (record_id, email, send_count, last_used, name_overridden, is_primary)
			VALUES (?, ?, 0, NULL, 1, 1)
		`, id, email); err != nil {
			return fmt.Errorf("failed to save contact address: %w", err)
		}
	case err != nil:
		return fmt.Errorf("failed to look up contact: %w", err)
	default:
		if _, err := tx.Exec(`
			UPDATE contact_records SET fn = ?, kind = 'manual', updated_at = ? WHERE id = ?
		`, name, now, id); err != nil {
			return fmt.Errorf("failed to update contact: %w", err)
		}
		if _, err := tx.Exec(`
			UPDATE contact_emails SET name_overridden = 1 WHERE record_id = ? AND email = ?
		`, id, email); err != nil {
			return fmt.Errorf("failed to protect contact name: %w", err)
		}
	}
	return tx.Commit()
}
