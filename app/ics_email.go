package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hkdb/aerion/internal/email"
	"github.com/hkdb/aerion/internal/logging"
	"github.com/hkdb/aerion/internal/message"
)

// Events from calendar files attached to an email, for "Add to calendar"
// (Emailio addition).

func isCalendarAttachment(att *message.Attachment) bool {
	switch strings.ToLower(att.ContentType) {
	case "text/calendar", "text/x-vcalendar", "application/ics":
		return true
	}
	return strings.EqualFold(filepath.Ext(att.Filename), ".ics")
}

// GetEmailEvents returns the events in the calendar files attached to an email.
// Files that can't be read are skipped: the email's text is used instead.
func (a *App) GetEmailEvents(messageID string) ([]*ICSEvent, error) {
	log := logging.WithComponent("app")

	siblings, err := a.attachmentStore.GetByMessage(messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get attachments: %w", err)
	}
	var files []*message.Attachment
	for _, att := range siblings {
		if isCalendarAttachment(att) && att.Size <= maxICSFileSize {
			files = append(files, att)
		}
	}
	if len(files) == 0 {
		return nil, nil
	}

	msg, err := a.messageStore.Get(messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}
	if msg == nil {
		return nil, fmt.Errorf("message not found: %s", messageID)
	}
	raw, err := a.syncEngine.FetchRawMessage(a.ctx, msg.AccountID, msg.FolderID, msg.UID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch message: %w", err)
	}

	downloader := email.NewAttachmentDownloader(a.paths.AttachmentsPath())
	var events []*ICSEvent
	seen := make(map[string]bool)
	for _, att := range files {
		content, err := downloader.ExtractPart(raw, email.PartRefFor(att, siblings))
		if err != nil {
			log.Warn().Err(err).Str("filename", att.Filename).Msg("Failed to extract calendar file")
			continue
		}
		for _, ev := range ParseAllICS(string(content)) {
			// Invitations often carry the same event in two parts
			key := fmt.Sprintf("%d|%d|%s", ev.StartUnix, ev.EndUnix, ev.Summary)
			if seen[key] {
				continue
			}
			seen[key] = true
			ev.FileName = att.Filename
			events = append(events, ev)
		}
	}
	return events, nil
}

// ParseAllICS reads every separate event in .ics data. Changed occurrences of a
// repeating event and unreadable events are left out.
func ParseAllICS(data string) []*ICSEvent {
	data = strings.ReplaceAll(strings.ReplaceAll(data, "\r\n", "\n"), "\r", "\n")

	// Split into the events and everything around them, such as time zones
	var around, current []string
	var blocks [][]string
	inEvent := false
	for _, line := range strings.Split(data, "\n") {
		switch tag := strings.ToUpper(strings.TrimSpace(line)); {
		case tag == "BEGIN:VEVENT":
			inEvent = true
			current = []string{line}
		case tag == "END:VEVENT" && inEvent:
			inEvent = false
			blocks = append(blocks, append(current, line))
		case tag == "END:VCALENDAR", strings.TrimSpace(line) == "":
			// added back below
		case inEvent:
			current = append(current, line)
		default:
			around = append(around, line)
		}
	}

	var events []*ICSEvent
	for i, block := range blocks {
		hasUID, override := false, false
		for _, line := range block {
			name := strings.ToUpper(line)
			hasUID = hasUID || strings.HasPrefix(name, "UID:") || strings.HasPrefix(name, "UID;")
			override = override || strings.HasPrefix(name, "RECURRENCE-ID")
		}
		if override && len(blocks) > 1 {
			continue
		}
		lines := append([]string{}, around...)
		lines = append(lines, block[0])
		if !hasUID {
			// Required by the reader, left out by some senders
			lines = append(lines, fmt.Sprintf("UID:emailio-%d", i))
		}
		lines = append(lines, block[1:]...)
		lines = append(lines, "END:VCALENDAR", "")
		if ev, err := ParseICS(strings.Join(lines, "\r\n")); err == nil {
			events = append(events, ev)
		}
	}
	return events
}
