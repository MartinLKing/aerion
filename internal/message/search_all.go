package message

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// allMailFolderFilter leaves out mail the user has thrown away, so it doesn't
// clutter a search across everything. Those folders can still be searched on
// their own with SearchConversations.
const allMailFolderFilter = "f.folder_type NOT IN ('trash', 'spam')"

// SearchConversationsAllMail searches every downloaded folder of every enabled
// account, except Trash and Spam. Unlike SearchConversationsUnifiedInbox, a thread
// can match in several folders, so each result opens from the folder that shows
// the most of it: see the folder_id subquery below.
func (s *Store) SearchConversationsAllMail(query string, offset, limit int, filter string) ([]*ConversationSearchResult, int, error) {
	if strings.TrimSpace(query) == "" {
		return nil, 0, nil
	}

	ftsQuery := prepareFTSQuery(query)

	countQuery := `
		SELECT COUNT(DISTINCT COALESCE(m.thread_id, m.id) || '-' || a.id)
		FROM messages m
		JOIN messages_fts fts ON m.rowid = fts.rowid
		INNER JOIN folders f ON m.folder_id = f.id AND ` + allMailFolderFilter + `
		INNER JOIN accounts a ON f.account_id = a.id AND a.enabled = 1
		WHERE messages_fts MATCH ?
	` + filterWhereClause(filter, "m.")
	var totalCount int
	if err := s.db.QueryRow(countQuery, ftsQuery).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("failed to count all-mail search results: %w", err)
	}
	if totalCount == 0 {
		return nil, 0, nil
	}

	// GetConversation shows the given folder plus Sent and Drafts, so opening a
	// thread from Sent would hide its other messages. The folder is chosen from
	// the whole thread rather than just the matching messages, preferring the
	// inbox, then any other folder, and Sent or Drafts last.
	searchQuery := `
		SELECT g.conv_thread_id, g.subject, g.snippet, g.from_name, g.message_count, g.unread_count,
		       g.has_attachments, g.is_starred, g.latest_date, g.message_ids, g.is_encrypted,
		       g.account_id, g.account_name, g.account_color,
		       pf.id, pf.name, pf.folder_type, g.participants_json
		FROM (
			SELECT
				COALESCE(m.thread_id, m.id) as conv_thread_id,
				MIN(m.subject) as subject,
				MAX(m.snippet) as snippet,
				MIN(m.from_name) as from_name,
				COUNT(*) as message_count,
				SUM(CASE WHEN m.is_read = 0 THEN 1 ELSE 0 END) as unread_count,
				MAX(CASE WHEN m.has_attachments = 1 THEN 1 ELSE 0 END) as has_attachments,
				MAX(CASE WHEN m.is_starred = 1 THEN 1 ELSE 0 END) as is_starred,
				MAX(m.date) as latest_date,
				GROUP_CONCAT(m.id) as message_ids,
				MAX(CASE WHEN m.smime_encrypted = 1 OR m.pgp_encrypted = 1 THEN 1 ELSE 0 END) as is_encrypted,
				a.id as account_id,
				a.name as account_name,
				a.color as account_color,
				json_group_array(json_object('name', m.from_name, 'email', m.from_email, 'date', m.date, 'snippet', m.snippet)) as participants_json
			FROM messages m
			JOIN messages_fts fts ON m.rowid = fts.rowid
			INNER JOIN folders f ON m.folder_id = f.id AND ` + allMailFolderFilter + `
			INNER JOIN accounts a ON f.account_id = a.id AND a.enabled = 1
			WHERE messages_fts MATCH ?
			GROUP BY COALESCE(m.thread_id, m.id), a.id` +
		filterHavingClause(filter, "m.") + `
			ORDER BY latest_date DESC
			LIMIT ? OFFSET ?
		) g
		INNER JOIN folders pf ON pf.id = (
			SELECT f.id
			FROM messages m
			INNER JOIN folders f ON m.folder_id = f.id AND ` + allMailFolderFilter + `
			WHERE m.account_id = g.account_id AND COALESCE(m.thread_id, m.id) = g.conv_thread_id
			ORDER BY CASE f.folder_type WHEN 'inbox' THEN 0 WHEN 'sent' THEN 2 WHEN 'drafts' THEN 2 ELSE 1 END, f.id
			LIMIT 1
		)
		ORDER BY g.latest_date DESC
	`

	rows, err := s.db.Query(searchQuery, ftsQuery, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search all mail: %w", err)
	}
	defer rows.Close()

	var results []*ConversationSearchResult
	for rows.Next() {
		c := &ConversationSearchResult{}
		var latestDateStr, snippet, fromName, messageIDsStr, participantsJSON sql.NullString

		if err := rows.Scan(
			&c.ThreadID,
			&c.Subject,
			&snippet,
			&fromName,
			&c.MessageCount,
			&c.UnreadCount,
			&c.HasAttachments,
			&c.IsStarred,
			&latestDateStr,
			&messageIDsStr,
			&c.IsEncrypted,
			&c.AccountID,
			&c.AccountName,
			&c.AccountColor,
			&c.FolderID,
			&c.FolderName,
			&c.FolderType,
			&participantsJSON,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan all-mail search result: %w", err)
		}

		if snippet.Valid {
			c.Snippet = snippet.String
		}
		if latestDateStr.Valid && latestDateStr.String != "" {
			c.LatestDate = parseTimeString(latestDateStr.String)
		}
		if messageIDsStr.Valid && messageIDsStr.String != "" {
			c.MessageIDs = strings.Split(messageIDsStr.String, ",")
		}
		if participantsJSON.Valid {
			var latestSnippet string
			c.Participants, latestSnippet = parseParticipantsJSON(participantsJSON.String)
			if latestSnippet != "" {
				c.Snippet = latestSnippet
			}
		}

		c.HighlightedSubject = highlightMatches(c.Subject, query)
		c.HighlightedSnippet = highlightMatches(c.Snippet, query)
		if fromName.Valid {
			c.HighlightedFromName = highlightMatches(fromName.String, query)
		}

		results = append(results, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to read all-mail search results: %w", err)
	}

	return results, totalCount, nil
}

// BackfillBodyTextFromHTML gives already-downloaded HTML-only messages a plain
// text version, so their bodies become searchable: updating body_text also
// updates the full-text index through the messages_fts_update trigger. Encrypted
// messages and drafts are left alone. Returns how many messages were updated.
func (s *Store) BackfillBodyTextFromHTML(ctx context.Context, toText func(html string) string) (int, error) {
	const batchSize = 200

	type pending struct {
		rowid int64
		html  string
	}

	updated := 0
	var lastRowID int64
	for {
		rows, err := s.db.QueryContext(ctx, `
			SELECT m.rowid, m.body_html
			FROM messages m
			INNER JOIN folders f ON m.folder_id = f.id
			WHERE m.rowid > ?
			  AND (m.body_text IS NULL OR m.body_text = '')
			  AND m.body_html IS NOT NULL AND m.body_html <> ''
			  AND m.smime_encrypted = 0 AND m.pgp_encrypted = 0
			  AND f.folder_type <> 'drafts'
			ORDER BY m.rowid
			LIMIT ?
		`, lastRowID, batchSize)
		if err != nil {
			return updated, fmt.Errorf("failed to find HTML-only messages: %w", err)
		}

		var batch []pending
		for rows.Next() {
			var p pending
			if err := rows.Scan(&p.rowid, &p.html); err != nil {
				rows.Close()
				return updated, fmt.Errorf("failed to read HTML-only message: %w", err)
			}
			batch = append(batch, p)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return updated, fmt.Errorf("failed to read HTML-only messages: %w", err)
		}
		if len(batch) == 0 {
			return updated, nil
		}

		for _, p := range batch {
			lastRowID = p.rowid
			text := toText(p.html)
			if strings.TrimSpace(text) == "" {
				continue
			}
			if _, err := s.db.ExecContext(ctx, `UPDATE messages SET body_text = ? WHERE rowid = ?`, text, p.rowid); err != nil {
				return updated, fmt.Errorf("failed to update message body text: %w", err)
			}
			updated++
		}

		// Leave room for sync and the UI between batches
		select {
		case <-ctx.Done():
			return updated, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}
