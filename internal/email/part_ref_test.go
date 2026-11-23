package email

import (
	"strings"
	"testing"

	"github.com/hkdb/aerion/internal/message"
)

// Same name throughout: two with Content-IDs, two plain of different size, and
// a second plain one of the same size, in a nested multipart.
var sameNameMessage = []byte(strings.ReplaceAll(`From: a@x.com
To: b@x.com
Subject: t
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="o"

--o
Content-Type: multipart/related; boundary="i"

--i
Content-Type: text/html

<p>hi</p>
--i
Content-Type: image/png
Content-ID: <one@x>
Content-Disposition: inline; filename="f.txt"

cid-one
--i
Content-Type: image/png
Content-ID: <two@x>
Content-Disposition: inline; filename="f.txt"

cid-two
--i--
--o
Content-Type: text/plain
Content-Disposition: attachment; filename="f.txt"

plain-long
--o
Content-Type: text/plain
Content-Disposition: attachment; filename="f.txt"

aaaa
--o
Content-Type: text/plain
Content-Disposition: attachment; filename="f.txt"

bbbb
--o--
`, "\n", "\r\n"))

func TestExtractPartSameName(t *testing.T) {
	atts := []*message.Attachment{
		{ID: "1", Filename: "f.txt", ContentID: "one@x", Size: 7},
		{ID: "2", Filename: "f.txt", ContentID: "two@x", Size: 7},
		{ID: "3", Filename: "f.txt", Size: 10},
		{ID: "4", Filename: "f.txt", Size: 4},
		{ID: "5", Filename: "f.txt", Size: 4},
	}
	want := []string{"cid-one", "cid-two", "plain-long", "aaaa", "bbbb"}
	d := NewAttachmentDownloader(t.TempDir())
	for i, att := range atts {
		got, err := d.ExtractPart(sameNameMessage, PartRefFor(att, atts))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want[i] {
			t.Errorf("attachment %s = %q, want %q", att.ID, got, want[i])
		}
	}

	// Files missing from the database don't shift the choice.
	got, err := d.ExtractPart(sameNameMessage, PartRefFor(atts[2], atts[2:3]))
	if err != nil || string(got) != "plain-long" {
		t.Errorf("partly stored message: %q, %v", got, err)
	}
	// Wrong stored details still give a file.
	if got, err := d.ExtractPart(sameNameMessage, PartRef{Filename: "f.txt", ContentID: "gone", Size: 999, Index: 50}); err != nil || string(got) != "cid-one" {
		t.Errorf("fallback: %q, %v", got, err)
	}
	if _, err := d.ExtractPart(sameNameMessage, PartRef{Filename: "other"}); err == nil {
		t.Error("an unknown name should fail")
	}
}
