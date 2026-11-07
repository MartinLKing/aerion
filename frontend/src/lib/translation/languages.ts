// Display names for translation languages, in the language itself so each is
// recognisable whatever the app's own language is.
const NATIVE_NAMES: Record<string, string> = {
  en: 'English',
  fr: 'Français',
  de: 'Deutsch',
  es: 'Español',
}

export function translationLanguageName(code: string): string {
  return NATIVE_NAMES[code] ?? code
}

// Name for a language Azure detected, which can be any language it knows.
export function detectedLanguageName(code: string, uiLocale = 'en'): string {
  if (!code) return ''
  if (NATIVE_NAMES[code]) return NATIVE_NAMES[code]
  try {
    return new Intl.DisplayNames([uiLocale], { type: 'language' }).of(code) ?? code
  } catch {
    return code
  }
}
