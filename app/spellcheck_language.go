package app

// GetSpellcheckActiveLanguage returns the composer's last chosen spell-check language.
func (a *App) GetSpellcheckActiveLanguage() (string, error) {
	return a.settingsStore.GetSpellcheckActiveLanguage()
}

// SetSpellcheckActiveLanguage remembers the composer's chosen spell-check language.
func (a *App) SetSpellcheckActiveLanguage(code string) error {
	return a.settingsStore.SetSpellcheckActiveLanguage(code)
}

// GetSpellcheckActiveLanguage returns the chosen spell-check language for the detached composer.
func (c *ComposerApp) GetSpellcheckActiveLanguage() (string, error) {
	return c.settingsStore.GetSpellcheckActiveLanguage()
}

// SetSpellcheckActiveLanguage remembers a spell-check language chosen in the detached composer.
func (c *ComposerApp) SetSpellcheckActiveLanguage(code string) error {
	return c.settingsStore.SetSpellcheckActiveLanguage(code)
}
