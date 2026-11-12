package sync

import (
	"fmt"
	"testing"

	"github.com/hkdb/aerion/internal/smtp"
)

// TestParseMultipartBody_ManyAttachmentsGetUniqueIDs guards against IDs that
// collide when attachments are parsed within one clock tick (Windows), which
// made the database keep only some of them.
func TestParseMultipartBody_ManyAttachmentsGetUniqueIDs(t *testing.T) {
	m := &smtp.ComposeMessage{
		From:     smtp.Address{Address: "a@example.com"},
		To:       []smtp.Address{{Address: "b@example.com"}},
		Subject:  "eight files",
		TextBody: "hi",
		HTMLBody: "<p>hi</p>",
	}
	const n = 8
	for i := 0; i < n; i++ {
		m.Attachments = append(m.Attachments, smtp.Attachment{
			Filename:    fmt.Sprintf("file%d.svg", i),
			ContentType: "image/svg+xml",
			Content:     []byte(fmt.Sprintf("<svg>%d</svg>", i)),
			ContentID:   fmt.Sprintf("image%d@aerion", i),
			Inline:      i%2 == 0,
		})
	}
	raw, err := m.ToRFC822()
	if err != nil {
		t.Fatal(err)
	}

	result := parseAttachments(t, raw)
	if len(result.Attachments) != n {
		t.Fatalf("got %d attachments, want %d", len(result.Attachments), n)
	}
	seen := map[string]bool{}
	for _, a := range result.Attachments {
		if seen[a.ID] {
			t.Fatalf("duplicate attachment ID %q", a.ID)
		}
		seen[a.ID] = true
	}
}
