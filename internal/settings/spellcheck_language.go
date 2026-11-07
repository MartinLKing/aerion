package settings

// KeySpellcheckActiveLanguage remembers the composer's spell-check language
// dropdown: a dictionary code, "all" for every language switched on in Settings,
// or "" if the user hasn't chosen yet.
const KeySpellcheckActiveLanguage = "spellcheck_active_language"

// GetSpellcheckActiveLanguage returns the composer's last chosen spell-check language.
func (s *Store) GetSpellcheckActiveLanguage() (string, error) {
	return s.Get(KeySpellcheckActiveLanguage)
}

// SetSpellcheckActiveLanguage remembers the composer's chosen spell-check language.
func (s *Store) SetSpellcheckActiveLanguage(code string) error {
	return s.Set(KeySpellcheckActiveLanguage, code)
}
