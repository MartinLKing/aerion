package message

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Reply / forward state for the conversation list (Emailio addition).
//
// A message counts as replied (or forwarded) when the server flag says so, or
// when the account's Sent folder holds a message written in response to it.
// The Sent check matters because Gmail and Outlook do not reliably set the
// \Answered flag for replies sent from their own web and phone apps.

// forwardSubjectCond matches the subject prefixes mail clients put on a
// forward (English, French, German, Spanish). A sent message that answers
// another one and carries such a prefix is a forward; otherwise it is a reply.
// SQLite's LIKE ignores ASCII case.
const forwardSubjectCond = `(r.subject LIKE 'fw:%' OR r.subject LIKE 'fwd:%' OR r.subject LIKE 'tr:%' OR r.subject LIKE 'wg:%' OR r.subject LIKE 'rv:%')`

// sentResponseExists is true when the Sent folder of the same account holds a
// message whose In-Reply-To points at message m. Message-IDs are stored with
// or without angle brackets depending on where they came from, so all three
// spellings are tried (each one is an index lookup).
const sentResponseExists = `EXISTS (
	SELECT 1 FROM messages r
	INNER JOIN folders rf ON rf.id = r.folder_id AND rf.folder_type = 'sent'
	WHERE r.account_id = m.account_id
	  AND r.in_reply_to IN (m.message_id, '<' || m.message_id || '>', TRIM(m.message_id, '<>'))
	  AND %s
)`

// replyStateChunk keeps the IN (...) list well under SQLite's parameter limit.
const replyStateChunk = 500

// fillReplyState sets IsAnswered and IsForwarded on each conversation from the
// newest of its messages, so the arrow goes away again when a new message
// arrives that has not been answered yet. It only looks at the conversations
// it is given (one page of the list), so the cost does not grow with the size
// of the folder. Failures are logged and otherwise ignored: the list must
// never break over an indicator.
func (s *Store) fillReplyState(convs []*Conversation) {
	owner := make(map[string]*Conversation)
	newest := make(map[*Conversation]time.Time)
	var ids []string
	for _, c := range convs {
		for _, id := range c.MessageIDs {
			owner[id] = c
			ids = append(ids, id)
		}
	}

	for start := 0; start < len(ids); start += replyStateChunk {
		end := start + replyStateChunk
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]

		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(chunk)), ",")
		args := make([]interface{}, len(chunk))
		for i, id := range chunk {
			args[i] = id
		}

		query := `
			SELECT m.id, m.date,
				CASE WHEN m.is_answered = 1 OR (COALESCE(m.message_id, '') != '' AND ` +
			fmt.Sprintf(sentResponseExists, "NOT "+forwardSubjectCond) + `) THEN 1 ELSE 0 END,
				CASE WHEN m.is_forwarded = 1 OR (COALESCE(m.message_id, '') != '' AND ` +
			fmt.Sprintf(sentResponseExists, forwardSubjectCond) + `) THEN 1 ELSE 0 END
			FROM messages m
			WHERE m.id IN (` + placeholders + `)`

		rows, err := s.db.Query(query, args...)
		if err != nil {
			s.log.Warn().Err(err).Msg("Failed to load reply state for conversations")
			return
		}
		for rows.Next() {
			var id string
			var dateStr sql.NullString
			var answered, forwarded bool
			if err := rows.Scan(&id, &dateStr, &answered, &forwarded); err != nil {
				s.log.Warn().Err(err).Msg("Failed to scan reply state")
				break
			}
			c := owner[id]
			if c == nil {
				continue
			}
			var date time.Time
			if dateStr.Valid && dateStr.String != "" {
				date = parseTimeString(dateStr.String)
			}
			// Keep the state of the newest message only.
			if prev, seen := newest[c]; seen && !date.After(prev) {
				continue
			}
			newest[c] = date
			c.IsAnswered = answered
			c.IsForwarded = forwarded
		}
		rows.Close()
	}
}

// fillSearchReplyState is fillReplyState for search results.
func (s *Store) fillSearchReplyState(results []*ConversationSearchResult) {
	convs := make([]*Conversation, len(results))
	for i, r := range results {
		convs[i] = &r.Conversation
	}
	s.fillReplyState(convs)
}

// MarkResponded flags every local copy of the message with the given
// Message-ID header as replied (or forwarded) and returns those copies, so the
// caller can pass the flag on to the mail server. Sent and Drafts copies are
// left alone.
func (s *Store) MarkResponded(accountID, rfcMessageID string, forwarded bool) ([]*Message, error) {
	bare := normalizeMessageID(rfcMessageID)
	if bare == "" {
		return nil, nil
	}

	rows, err := s.db.Query(`
		SELECT m.id FROM messages m
		INNER JOIN folders f ON f.id = m.folder_id
		WHERE m.account_id = ? AND m.message_id IN (?, ?)
		  AND f.folder_type NOT IN ('sent', 'drafts')
	`, accountID, bare, "<"+bare+">")
	if err != nil {
		return nil, fmt.Errorf("failed to find original message: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("failed to scan original message: %w", err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if len(ids) == 0 {
		return nil, nil
	}

	column := "is_answered"
	if forwarded {
		column = "is_forwarded"
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	if _, err := s.db.Exec("UPDATE messages SET "+column+" = 1 WHERE id IN ("+placeholders+")", args...); err != nil {
		return nil, fmt.Errorf("failed to mark message as responded: %w", err)
	}

	return s.GetByIDs(ids)
}
