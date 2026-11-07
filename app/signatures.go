package app

import "github.com/hkdb/aerion/internal/settings"

// ============================================================================
// Named signature API - Exposed to frontend via Wails bindings
// ============================================================================

// GetNamedSignatures returns all named signatures.
func (a *App) GetNamedSignatures() ([]settings.NamedSignature, error) {
	return a.settingsStore.GetNamedSignatures()
}

// SaveNamedSignature creates or updates a named signature and returns it.
func (a *App) SaveNamedSignature(signature settings.NamedSignature) (settings.NamedSignature, error) {
	return a.settingsStore.SaveNamedSignature(signature)
}

// DeleteNamedSignature removes a named signature.
func (a *App) DeleteNamedSignature(id string) error {
	return a.settingsStore.DeleteNamedSignature(id)
}

// GetAccountSignatureDefault returns the account's default named signature ID, or "".
func (a *App) GetAccountSignatureDefault(accountID string) (string, error) {
	return a.settingsStore.GetAccountSignatureDefault(accountID)
}

// SetAccountSignatureDefault sets the account's default named signature. An empty ID clears it.
func (a *App) SetAccountSignatureDefault(accountID, signatureID string) error {
	return a.settingsStore.SetAccountSignatureDefault(accountID, signatureID)
}

// GetNamedSignatures returns all named signatures for the detached composer.
func (c *ComposerApp) GetNamedSignatures() ([]settings.NamedSignature, error) {
	return c.settingsStore.GetNamedSignatures()
}

// GetAccountSignatureDefault returns the account's default named signature ID for the detached composer.
func (c *ComposerApp) GetAccountSignatureDefault(accountID string) (string, error) {
	return c.settingsStore.GetAccountSignatureDefault(accountID)
}
