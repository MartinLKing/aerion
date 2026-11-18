package app

import (
	"fmt"

	"github.com/hkdb/aerion/internal/draft"
	"github.com/hkdb/aerion/internal/folder"
	"github.com/hkdb/aerion/internal/logging"
	"github.com/hkdb/aerion/internal/message"
	"github.com/hkdb/aerion/internal/smtp"
)

// Editing an existing draft (Emailio fix).
//
// The Drafts folder lists drafts as messages, but the composer saves and
// deletes by the local draft ID. Handing it the message ID made the first
// autosave create a brand-new draft and left the original in Drafts forever,
// even after sending. OpenDraftForEdit resolves the real draft ID first, and
// takes over drafts written elsewhere (webmail, phone) so that autosave
// replaces them and sending deletes them.

// EditableDraft is a draft ready to open in the composer.
type EditableDraft struct {
	DraftID   string               `json:"draftId"`
	AccountID string               `json:"accountId"`
	Message   *smtp.ComposeMessage `json:"message"`
}

// OpenDraftForEdit returns the draft behind a draft ID or a Drafts-folder
// message ID, creating a local draft for one written outside Emailio.
func (a *App) OpenDraftForEdit(id string) (*EditableDraft, error) {
	log := logging.WithComponent("app.draftEdit")

	if d, err := a.draftStore.Get(id); err != nil {
		return nil, err
	} else if d != nil {
		return &EditableDraft{DraftID: d.ID, AccountID: d.AccountID, Message: a.draftToComposeMessage(d)}, nil
	}

	msg, err := a.messageStore.Get(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}
	if msg == nil {
		return nil, fmt.Errorf("draft not found")
	}

	if d, err := a.draftStore.GetByIMAPUID(msg.FolderID, msg.UID); err != nil {
		return nil, err
	} else if d != nil {
		return &EditableDraft{DraftID: d.ID, AccountID: d.AccountID, Message: a.draftToComposeMessage(d)}, nil
	}

	// Written outside Emailio: only take over real drafts, never a message
	// from another folder.
	if f, _ := a.folderStore.Get(msg.FolderID); f == nil || f.Type != folder.TypeDrafts {
		return nil, fmt.Errorf("this message is not a draft")
	}

	cm := a.serverDraftToComposeMessage(msg)
	enc, err := a.draftOps.encryptDraftBody(msg.AccountID, cm.From.Address, *cm)
	if err != nil {
		return nil, err
	}
	d, err := a.draftOps.saveDraftToDB(msg.AccountID, nil, *cm, enc)
	if err != nil {
		return nil, err
	}
	// Point the new local draft at the copy already on the server, so the
	// next autosave replaces it and sending or discarding deletes it.
	if err := a.draftStore.UpdateSyncStatus(d.ID, draft.SyncStatusSynced, msg.UID, msg.FolderID, ""); err != nil {
		return nil, err
	}

	log.Info().Str("messageID", msg.ID).Str("draftID", d.ID).Uint32("uid", msg.UID).Msg("Took over draft written outside Emailio")
	return &EditableDraft{DraftID: d.ID, AccountID: msg.AccountID, Message: cm}, nil
}

// serverDraftToComposeMessage builds composer content from a draft found
// only on the server, fetching its body and attachments if needed.
func (a *App) serverDraftToComposeMessage(msg *message.Message) *smtp.ComposeMessage {
	log := logging.WithComponent("app.draftEdit")

	if !msg.BodyFetched && a.syncEngine != nil {
		if updated, err := a.syncEngine.FetchMessageBody(a.ctx, msg.AccountID, msg.ID); err == nil && updated != nil {
			msg = updated
		} else if err != nil {
			log.Warn().Err(err).Msg("Failed to fetch draft body")
		}
	}

	cm := a.messageToComposeMessage(msg)
	cm.From = smtp.Address{Name: msg.FromName, Address: msg.FromEmail}
	a.fetchForwardAttachments(log, msg, &cm.Attachments)
	return cm
}
