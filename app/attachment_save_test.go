package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hkdb/aerion/internal/message"
)

func TestUniqueSavePath(t *testing.T) {
	dir := t.TempDir()
	for _, want := range []string{"f.html", "f (2).html", "f (3).html"} {
		path := uniqueSavePath(dir, "f.html")
		if filepath.Base(path) != want {
			t.Fatalf("got %s, want %s", filepath.Base(path), want)
		}
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{`..\..\evil.exe`, "../evil.exe", `C:\x\evil.exe`, "..", "", "a:b?.txt"} {
		if path := uniqueSavePath(dir, name); filepath.Dir(path) != dir {
			t.Errorf("%q escaped to %s", name, path)
		}
	}
}

// Inline files stay in the list: the frontend decides which ones the body shows.
func TestDecryptedAttachmentListKeepsInline(t *testing.T) {
	got := buildDecryptedAttachmentList([]*message.Attachment{
		{Filename: "logo.png", IsInline: true, ContentID: "c"},
		{Filename: "doc.pdf", IsInline: true},
	})
	if len(got) != 2 || got[0].Index != 0 || got[1].Index != 1 || !got[0].IsInline {
		t.Fatalf("got %+v", got)
	}
}
