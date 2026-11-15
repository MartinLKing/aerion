package app

import (
	"strings"
	"time"

	"github.com/hkdb/aerion/internal/logging"
	"github.com/hkdb/aerion/internal/message"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Reply / forward indicator (Emailio addition). See internal/message/reply_state.go.

// isForwardSubject reports whether a sent subject carries a forward prefix.
// Keep in step with forwardSubjectCond in internal/message/reply_state.go.
func isForwardSubject(subject string) bool {
	s := strings.ToLower(strings.TrimSpace(subject))
	for _, prefix := range []string{"fw:", "fwd:", "tr:", "wg:", "rv:"} {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

// markOriginalResponded runs after a message was sent. If the message answered
// or forwarded another one, the original is flagged as replied / forwarded
// locally right away (so the arrow shows without waiting for a sync), the list
// is told to update, and the flag is passed on to the mail server in the
// background. Everything here is best-effort: the send already succeeded.
func (a *App) markOriginalResponded(accountID, inReplyTo, subject string) {
	if strings.TrimSpace(inReplyTo) == "" {
		return
	}
	log := logging.WithComponent("app.replyState")

	forwarded := isForwardSubject(subject)
	originals, err := a.messageStore.MarkResponded(accountID, inReplyTo, forwarded)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to mark original message as responded")
		return
	}
	if len(originals) == 0 {
		return
	}

	ids := make([]string, len(originals))
	byFolder := make(map[string][]*message.Message)
	for i, m := range originals {
		ids[i] = m.ID
		byFolder[m.FolderID] = append(byFolder[m.FolderID], m)
	}

	wailsRuntime.EventsEmit(a.ctx, "messages:respondedChanged", map[string]interface{}{
		"messageIds":  ids,
		"isAnswered":  !forwarded,
		"isForwarded": forwarded,
	})

	flagType := "answered"
	if forwarded {
		flagType = "forwarded"
	}
	go func() {
		defer recoverPanic("app.replyState", "sync responded flag to IMAP")
		for folderID, msgs := range byFolder {
			var err error
			for attempt := 1; attempt <= 3; attempt++ {
				err = a.syncFlagsToIMAP(msgs, folderID, flagType, true)
				if err == nil {
					break
				}
				time.Sleep(time.Duration(attempt) * time.Second)
			}
			if err != nil {
				// Not every server accepts the forwarded keyword. The arrow
				// still shows, derived from the Sent folder.
				log.Warn().Err(err).Str("folderID", folderID).Str("flag", flagType).Msg("Failed to sync responded flag to IMAP")
			}
		}
	}()
}
