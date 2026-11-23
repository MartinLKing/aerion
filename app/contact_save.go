package app

import (
	"strings"

	"github.com/hkdb/aerion/internal/account"
	"github.com/hkdb/aerion/internal/contact"
	"github.com/hkdb/aerion/internal/ipc"
)

// Saving a contact from an email, in the main window and the pop-out composer
// (Emailio addition).

func savedState(accounts *account.Store, contacts *contact.Store, email string) (contact.SavedState, error) {
	if ownAddresses(accounts)[strings.ToLower(strings.TrimSpace(email))] {
		return contact.SavedState{State: contact.SavedOwn}, nil
	}
	return contacts.GetSavedState(email)
}

// GetContactSavedState reports whether an address is a contact.
func (a *App) GetContactSavedState(email string) (contact.SavedState, error) {
	return savedState(a.accountStore, a.contactStore, email)
}

// SaveContact saves an address as a local contact, or renames it.
func (a *App) SaveContact(email, name string) error {
	if err := a.contactStore.SaveLocal(email, name); err != nil {
		return err
	}
	a.emitContactsChanged()
	return nil
}

// GetContactSavedState reports whether an address is a contact.
func (c *ComposerApp) GetContactSavedState(email string) (contact.SavedState, error) {
	return savedState(c.accountStore, c.contactStore, email)
}

// SaveContact saves an address as a local contact, or renames it.
func (c *ComposerApp) SaveContact(email, name string) error {
	if err := c.contactStore.SaveLocal(email, name); err != nil {
		return err
	}
	if c.ipcClient != nil {
		if msg, err := ipc.NewMessage(ipc.TypeContactsChanged, struct{}{}); err == nil {
			_ = c.ipcClient.Send(msg)
		}
	}
	return nil
}
