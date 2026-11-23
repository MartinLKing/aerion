package message

import (
	"database/sql"
	"fmt"
	"time"
)

// SentRecipients holds the raw address lists of one message (Emailio).
type SentRecipients struct {
	Date    time.Time
	ToList  string
	CcList  string
	BccList string
}

// ListRecipientsSince returns the recipients of messages in a folder dated
// after since. Used to learn contacts from the Sent folder.
func (s *Store) ListRecipientsSince(folderID string, since time.Time) ([]SentRecipients, error) {
	// Dates are text in mixed time zones: narrow by day here, compare below.
	day := ""
	if !since.IsZero() {
		day = since.UTC().AddDate(0, 0, -2).Format("2006-01-02")
	}
	rows, err := s.db.Query(`
		SELECT date, to_list, cc_list, bcc_list FROM messages
		WHERE folder_id = ? AND date >= ?
	`, folderID, day)
	if err != nil {
		return nil, fmt.Errorf("failed to list sent recipients: %w", err)
	}
	defer rows.Close()

	var out []SentRecipients
	for rows.Next() {
		var date, to, cc, bcc sql.NullString
		if err := rows.Scan(&date, &to, &cc, &bcc); err != nil {
			return nil, fmt.Errorf("failed to scan sent recipients: %w", err)
		}
		d := parseTimeString(date.String)
		if d.IsZero() || !d.After(since) {
			continue
		}
		out = append(out, SentRecipients{Date: d, ToList: to.String, CcList: cc.String, BccList: bcc.String})
	}
	return out, rows.Err()
}
