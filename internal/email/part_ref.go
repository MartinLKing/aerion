package email

import (
	"github.com/hkdb/aerion/internal/message"
)

// PartRef says which of an email's files is meant (Emailio). The name alone is
// not enough: one email can hold several different files with the same name.
type PartRef struct {
	Filename  string
	ContentID string
	Size      int
	// Index is the file's position among those that share its name, Content-ID
	// and size, for telling the last ones apart.
	Index int
}

// PartRefFor builds the reference for att. siblings are all attachments of the
// same message; those sharing a name must be in the order of the email.
func PartRefFor(att *message.Attachment, siblings []*message.Attachment) PartRef {
	ref := PartRef{Filename: att.Filename, ContentID: att.ContentID, Size: att.Size}
	for _, s := range siblings {
		if s.ID == att.ID {
			break
		}
		if s.Filename == att.Filename && s.ContentID == att.ContentID && s.Size == att.Size {
			ref.Index++
		}
	}
	return ref
}

// partCandidate is a part whose name matches a reference.
type partCandidate struct {
	contentID string
	size      int // as measured when the message was synced
	content   []byte
}

// pickPart chooses among same-named parts: by Content-ID, then size, then
// position. A criterion that matches nothing is ignored, so a stored value
// that is off never makes a file unavailable. candidates must not be empty.
func pickPart(candidates []partCandidate, ref PartRef) []byte {
	narrow := func(keep func(partCandidate) bool) {
		var kept []partCandidate
		for _, c := range candidates {
			if keep(c) {
				kept = append(kept, c)
			}
		}
		if len(kept) > 0 {
			candidates = kept
		}
	}
	// Also for an empty Content-ID: the file wanted is then one without.
	narrow(func(c partCandidate) bool { return c.contentID == ref.ContentID })
	if ref.Size > 0 {
		narrow(func(c partCandidate) bool { return c.size == ref.Size })
	}
	if ref.Index < len(candidates) {
		return candidates[ref.Index].content
	}
	return candidates[0].content
}
