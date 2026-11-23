package app

import (
	"testing"

	"github.com/hkdb/aerion/internal/message"
)

func TestShownInBody(t *testing.T) {
	html := `<img src="cid:logo@x">`
	stored := map[string]string{"logo@x": "data:image/png;base64,AA==", "photo@x": "data:image/png;base64,AA=="}
	for name, c := range map[string]struct {
		att  message.Attachment
		want bool
	}{
		"displayed logo":        {message.Attachment{IsInline: true, ContentID: "logo@x", ContentType: "image/png"}, true},
		"photo without an ID":   {message.Attachment{IsInline: true, ContentType: "image/jpeg"}, false},
		"photo not in the text": {message.Attachment{IsInline: true, ContentID: "photo@x", ContentType: "image/png"}, false},
		"PDF":                   {message.Attachment{IsInline: true, ContentID: "logo@x", ContentType: "application/pdf"}, false},
		"image without content": {message.Attachment{IsInline: true, ContentID: "big@x", ContentType: "image/png"}, false},
		"normal attachment":     {message.Attachment{ContentType: "image/png"}, false},
	} {
		if got := shownInBody(&c.att, html, stored); got != c.want {
			t.Errorf("%s: %v, want %v", name, got, c.want)
		}
	}
}

// The pop-out composer builds replies with the main window's logic: embedded
// images come along, and the reply goes to Reply-To.
func TestPopOutPrepareReply(t *testing.T) {
	a, db := newDraftEditTestApp(t)
	m := &message.Message{
		ID: "orig", AccountID: "acct", FolderID: "inbox", UID: 1, MessageID: "orig@x",
		Subject: "Music", FromEmail: "from@x.com", ReplyTo: "answers@x.com", BodyFetched: true,
		BodyHTML: `<p>Hi <img src="cid:logo@x"></p>`,
	}
	if err := a.messageStore.Create(m); err != nil {
		t.Fatal(err)
	}
	for _, att := range []*message.Attachment{
		{ID: "a1", MessageID: "orig", Filename: "logo.png", ContentType: "image/png", ContentID: "logo@x", IsInline: true, Size: 2, Content: []byte("xx")},
		{ID: "a2", MessageID: "orig", Filename: "photo.jpg", ContentType: "image/jpeg", IsInline: true, Size: 2, Content: []byte("yy")},
	} {
		if err := a.attachmentStore.Create(att); err != nil {
			t.Fatal(err)
		}
	}

	c := &ComposerApp{db: db, accountStore: a.accountStore, folderStore: a.folderStore, messageStore: a.messageStore}
	got, err := c.prepareReply("orig", "reply")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.To) != 1 || got.To[0].Address != "answers@x.com" {
		t.Errorf("to = %+v", got.To)
	}
	if got.InReplyTo != "<orig@x>" || len(got.References) != 1 {
		t.Errorf("threading = %q %v", got.InReplyTo, got.References)
	}
	if len(got.Attachments) != 1 || got.Attachments[0].Filename != "logo.png" || !got.Attachments[0].Inline {
		t.Errorf("attachments = %+v", got.Attachments)
	}
}
