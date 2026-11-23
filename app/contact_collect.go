package app

import (
	"strings"
	"sync"
	"time"

	"github.com/hkdb/aerion/internal/account"
	"github.com/hkdb/aerion/internal/folder"
	"github.com/hkdb/aerion/internal/logging"
	"github.com/hkdb/aerion/internal/smtp"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Collecting contacts from sent mail (Emailio addition).

// learnContactsKey + account ID holds the date up to which that account's Sent
// mail has been read. Older mail is never read again, so a contact the user
// deletes is not brought back by it.
const learnContactsKey = "emailio_contacts_learned_until:"

var learnContactsMu sync.Mutex

// ownAddresses returns every address of the user, lowercased.
func ownAddresses(store *account.Store) map[string]bool {
	own := make(map[string]bool)
	accounts, err := store.List()
	if err != nil {
		return own
	}
	for _, acc := range accounts {
		own[strings.ToLower(strings.TrimSpace(acc.Email))] = true
		identities, _ := store.GetIdentities(acc.ID)
		for _, id := range identities {
			own[strings.ToLower(strings.TrimSpace(id.Email))] = true
		}
	}
	return own
}

// collectRecipients saves everyone a message was just sent to as a contact.
// The send already succeeded, so failures are only logged.
func (ops *composeOps) collectRecipients(msg *smtp.ComposeMessage) {
	log := logging.WithComponent("composeOps")
	seen := ownAddresses(ops.accountStore)
	for _, list := range [][]smtp.Address{msg.To, msg.Cc, msg.Bcc} {
		for _, addr := range list {
			email := strings.ToLower(strings.TrimSpace(addr.Address))
			if email == "" || seen[email] {
				continue
			}
			seen[email] = true
			if err := ops.contactStore.AddOrUpdate(email, addr.Name); err != nil {
				log.Warn().Err(err).Msg("Failed to save recipient as contact")
			}
		}
	}
}

// emitContactsChanged makes an open Contacts view reload.
func (a *App) emitContactsChanged() {
	wailsRuntime.EventsEmit(a.ctx, "contacts:changed", map[string]string{"sourceId": "local"})
}

// learnContactsAfterSync reads new mail in a Sent folder that just synced, so
// people written to from webmail or a phone become contacts too.
func (a *App) learnContactsAfterSync(accountID, folderID string) {
	sent, err := a.GetSpecialFolder(accountID, folder.TypeSent)
	if err != nil || sent == nil || sent.ID != folderID {
		return
	}
	if a.learnContacts(accountID, folderID) > 0 {
		a.emitContactsChanged()
	}
}

// learnContacts does the reading and returns how many contacts were added.
func (a *App) learnContacts(accountID, folderID string) int {
	learnContactsMu.Lock()
	defer learnContactsMu.Unlock()
	log := logging.WithComponent("app.contacts")

	value, err := a.settingsStore.Get(learnContactsKey + accountID)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read Sent marker")
		return 0
	}
	var since time.Time
	if value != "" {
		if since, err = time.Parse(time.RFC3339, value); err != nil {
			log.Warn().Err(err).Msg("Bad Sent marker")
			return 0
		}
	}

	messages, err := a.messageStore.ListRecipientsSince(folderID, since)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read Sent recipients")
		return 0
	}
	if len(messages) == 0 {
		return 0
	}

	own := ownAddresses(a.accountStore)
	now := time.Now()
	until := since
	added := 0
	for _, m := range messages {
		at := m.Date
		if at.After(now) {
			at = now // a wrong clock must not pin someone to the top
		}
		if at.After(until) {
			until = at
		}
		for _, list := range []string{m.ToList, m.CcList, m.BccList} {
			for _, addr := range parseAddressList(list) {
				if own[strings.ToLower(strings.TrimSpace(addr.Address))] {
					continue
				}
				isNew, err := a.contactStore.RecordSent(addr.Address, addr.Name, at)
				if err != nil {
					log.Warn().Err(err).Msg("Failed to learn contact from Sent")
					return added // marker not moved: retried on the next sync
				}
				if isNew {
					added++
				}
			}
		}
	}

	// Round up: the marker keeps whole seconds and must not fall before the
	// newest message.
	until = until.Add(time.Second - time.Nanosecond).Truncate(time.Second)
	if err := a.settingsStore.Set(learnContactsKey+accountID, until.UTC().Format(time.RFC3339)); err != nil {
		log.Warn().Err(err).Msg("Failed to save Sent marker")
	}
	if added > 0 {
		log.Info().Int("count", added).Msg("Added contacts from Sent mail")
	}
	return added
}
