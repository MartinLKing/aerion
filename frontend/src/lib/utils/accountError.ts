// Turns a raw account sync error into something readable (Emailio addition).
// Used by the red exclamation popup in the sidebar.

export type AccountErrorKind = 'auth' | 'certificate' | 'network' | 'timeout' | 'unknown'

/**
 * Picks the part of a sync error that belongs to one account and strips the
 * wrapper text. "Sync all" reports every failed account in one string:
 * "sync errors: a@x.com: reason; b@y.com: reason".
 */
export function accountErrorDetail(raw: string | null | undefined, email: string): string {
  if (!raw) return ''
  let text = raw.replace(/^Error:\s*/, '').trim()

  if (text.startsWith('sync errors:')) {
    const parts = text.slice('sync errors:'.length).split('; ')
    const marker = email + ':'
    const start = parts.findIndex((p) => p.trim().startsWith(marker))
    if (start >= 0) {
      // A reason can itself contain "; " (several folders failed), so take
      // everything up to the next account's part.
      let end = parts.length
      for (let i = start + 1; i < parts.length; i++) {
        if (/^\s*(\S+@\S+|contacts):/.test(parts[i])) {
          end = i
          break
        }
      }
      text = parts.slice(start, end).join('; ').trim().slice(marker.length).trim()
    }
  }
  return text
}

/** Rough cause of an error, from the wording mail servers and Go use. */
export function classifyAccountError(detail: string): AccountErrorKind {
  const t = detail.toLowerCase()
  if (/oauth|token|invalid_grant|authenticat|unauthorized|invalid credentials|login failed|password/.test(t)) {
    return 'auth'
  }
  if (/certificate|x509/.test(t)) return 'certificate'
  if (/no such host|connection refused|unreachable|dial tcp|connectex|connection reset|broken pipe|forcibly closed|failed to get imap connection|\beof\b|lookup /.test(t)) {
    return 'network'
  }
  if (/timeout|timed out|deadline exceeded/.test(t)) return 'timeout'
  return 'unknown'
}
