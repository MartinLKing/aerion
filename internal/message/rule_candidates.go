package message

import (
	"fmt"

	"github.com/hkdb/aerion/internal/rules"
)

// ListRuleCandidates returns the messages in a folder with a UID above afterUID,
// oldest first, with the fields filter rules need. Requires uid > 0, which also
// skips the temporary negative UIDs of messages that are mid-move.
func (s *Store) ListRuleCandidates(folderID string, afterUID int64) ([]rules.Candidate, error) {
	rows, err := s.db.Query(`
		SELECT id, COALESCE(message_id, ''), uid, COALESCE(from_name, ''), COALESCE(from_email, ''),
		       COALESCE(subject, ''), is_read
		FROM messages
		WHERE folder_id = ? AND uid > ? AND uid > 0
		ORDER BY uid
	`, folderID, afterUID)
	if err != nil {
		return nil, fmt.Errorf("failed to list rule candidates: %w", err)
	}
	defer rows.Close()

	var candidates []rules.Candidate
	for rows.Next() {
		var c rules.Candidate
		if err := rows.Scan(&c.ID, &c.MessageID, &c.UID, &c.FromName, &c.FromEmail, &c.Subject, &c.IsRead); err != nil {
			return nil, fmt.Errorf("failed to read rule candidate: %w", err)
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read rule candidates: %w", err)
	}
	return candidates, nil
}

// MaxUID returns the highest real UID in a folder, or 0 if it has no messages.
func (s *Store) MaxUID(folderID string) (int64, error) {
	var uid int64
	err := s.db.QueryRow(`SELECT COALESCE(MAX(uid), 0) FROM messages WHERE folder_id = ?`, folderID).Scan(&uid)
	if err != nil {
		return 0, fmt.Errorf("failed to get max UID: %w", err)
	}
	if uid < 0 {
		return 0, nil
	}
	return uid, nil
}
