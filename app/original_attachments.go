package app

import (
	"strings"

	"github.com/hkdb/aerion/internal/email"
	"github.com/hkdb/aerion/internal/logging"
	"github.com/hkdb/aerion/internal/message"
	"github.com/hkdb/aerion/internal/smtp"
	syncpkg "github.com/hkdb/aerion/internal/sync"
)

// Attachments of a message being replied to, forwarded or edited as a draft
// (Emailio).

// shownInBody reports whether the HTML displays this attachment by itself, so
// it has to travel as an embedded image. The inline mark alone says little:
// Apple Mail puts it on attached photos and PDFs too. stored is the attachment
// content kept for display, as a data URL. Keep in step with notShownInBody in
// frontend/src/lib/utils/inlineAttachments.ts.
func shownInBody(att *message.Attachment, html string, stored map[string]string) bool {
	return att.IsInline && att.ContentID != "" && stored[att.ContentID] != "" &&
		strings.HasPrefix(att.ContentType, "image/") &&
		quotedHTMLReferencesCID(html, att.ContentID)
}

// originalAttachments returns the attachments of msg to put in the composer:
// the images its HTML displays, and with includeFiles all other files as
// normal attachments. Failures are logged; what can be had is returned.
func (a *App) originalAttachments(msg *message.Message, html string, includeFiles bool) []smtp.Attachment {
	log := logging.WithComponent("app")

	atts, err := a.attachmentStore.GetByMessage(msg.ID)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get attachments of original message")
		return nil
	}
	stored, err := a.attachmentStore.GetInlineByMessage(msg.ID)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get inline attachments of original message")
	}

	var out []smtp.Attachment
	var files []*message.Attachment
	embedded := make(map[string]bool)
	for _, att := range atts {
		if !shownInBody(att, html, stored) {
			files = append(files, att)
			continue
		}
		if embedded[att.ContentID] {
			continue
		}
		embedded[att.ContentID] = true
		contentType, b64 := parseDataURL(stored[att.ContentID])
		if b64 == "" {
			continue
		}
		out = append(out, smtp.Attachment{
			ContentBase64: b64,
			ContentType:   contentType,
			ContentID:     att.ContentID,
			Inline:        true,
			// The real filename, not the Content-ID: Outlook-style ids look like
			// forbidden ".com" files and bounced whole replies (#370).
			Filename: sanitizeAttachmentFilename(att.Filename),
		})
	}
	if !includeFiles || len(files) == 0 {
		return out
	}

	raw, fetchErr := a.syncEngine.FetchRawMessage(a.ctx, msg.AccountID, msg.FolderID, msg.UID)
	if fetchErr != nil {
		log.Warn().Err(fetchErr).Msg("Failed to fetch original message for its attachments (offline?)")
	}
	downloader := email.NewAttachmentDownloader(a.paths.AttachmentsPath())
	for _, att := range files {
		file := smtp.Attachment{
			ContentType: att.ContentType,
			Filename:    sanitizeAttachmentFilename(att.Filename),
		}
		if fetchErr == nil {
			file.Content, err = downloader.ExtractPart(raw, email.PartRefFor(att, atts))
			if err != nil {
				log.Warn().Err(err).Str("filename", att.Filename).Msg("Failed to extract original attachment")
			}
		}
		if len(file.Content) == 0 && att.ContentID != "" {
			_, file.ContentBase64 = parseDataURL(stored[att.ContentID])
		}
		if len(file.Content) == 0 && file.ContentBase64 == "" {
			continue
		}
		out = append(out, file)
	}
	return out
}

// prepareReply builds a reply or forward in the pop-out composer with the main
// window's logic. The simplified copy it replaces left out attachments,
// embedded images and Reply-To, and loaded remote images.
func (c *ComposerApp) prepareReply(messageID, mode string) (*smtp.ComposeMessage, error) {
	attachments := message.NewAttachmentStore(c.db)
	main := &App{
		ctx:             c.ctx,
		paths:           c.paths,
		accountStore:    c.accountStore,
		messageStore:    c.messageStore,
		attachmentStore: attachments,
		syncEngine:      syncpkg.NewEngine(c.imapPool, c.accountStore, c.folderStore, c.messageStore, attachments),
	}
	return main.PrepareReply(messageID, mode)
}
