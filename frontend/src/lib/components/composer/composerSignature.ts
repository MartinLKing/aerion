/**
 * Signature management utilities for the email composer
 */
// @ts-ignore - Wails generated imports
import { account } from '../../../../wailsjs/go/models'

// Re-export the Identity type for use in other files
export type Identity = account.Identity

export type ComposeMode = 'new' | 'reply' | 'reply-all' | 'forward'

/**
 * Signature marker - used to identify where signature starts for removal/swapping
 * Using three zero-width spaces as an invisible marker that TipTap preserves in text nodes
 */
export const SIGNATURE_MARKER = '\u200B\u200B\u200B'

/**
 * Build signature HTML from identity settings
 */
export function buildSignatureHtml(identity: Identity): string {
  if (!identity.signatureEnabled) return ''
  if (!identity.signatureHtml) return ''

  return wrapSignatureHtml(identity.signatureHtml, identity.signatureSeparator)
}

/**
 * Build insertable HTML for any signature: adds the marker used to find it
 * again later, plus the optional "-- " separator line.
 */
export function wrapSignatureHtml(signatureHtml: string, separator: boolean): string {
  if (!signatureHtml) return ''

  // Add separator line if enabled (with marker at the start)
  if (separator) {
    return `<p>${SIGNATURE_MARKER}-- </p>` + signatureHtml
  }

  // Inject marker into the first element of the signature
  // This ensures TipTap preserves it as text content
  const html = signatureHtml.replace(/^(<p[^>]*>)/, `$1${SIGNATURE_MARKER}`)
  // If signature doesn't start with <p>, wrap the marker
  if (!html.includes(SIGNATURE_MARKER)) {
    return `<p>${SIGNATURE_MARKER}</p>` + signatureHtml
  }
  return html
}

/**
 * Text the user has written above quoted content, followed by the spacing that
 * goes before a signature. Trailing blank lines are dropped so that swapping
 * signatures repeatedly doesn't add more each time.
 */
function typedContentBeforeSignature(typed: string): string {
  const trimmed = typed.replace(/(<p[^>]*>(\s|<br[^>]*>)*<\/p>\s*)+$/i, '')
  // Nothing typed: typing area + blank line below content
  if (!trimmed.trim()) return '<p></p><p></p>'
  return trimmed + '<p></p>'
}

/**
 * The part of the content above any quoted text (citation line, forwarded
 * message header or blockquote). Quoted history can hold signature markers from
 * earlier replies, so only this part reliably shows the current signature.
 */
export function getPreQuoteContent(content: string): string {
  const quoteIdx = content.indexOf('<blockquote')
  const wroteIdx = content.search(/wrote:\s*(<br[^>]*>)?\s*<\/p>/i)
  const fwdIdx = content.indexOf('---------- Forwarded message ----------')
  const boundaries = [quoteIdx, wroteIdx, fwdIdx].filter(i => i > -1)
  const quoteBoundary = boundaries.length > 0 ? Math.min(...boundaries) : content.length
  return content.substring(0, quoteBoundary)
}

/**
 * Check if signature should be appended for the given compose mode
 */
export function shouldAppendSignature(identity: Identity, mode: ComposeMode): boolean {
  if (!identity.signatureEnabled) return false

  switch (mode) {
    case 'new':
      return identity.signatureForNew
    case 'reply':
    case 'reply-all':
      return identity.signatureForReply
    case 'forward':
      return identity.signatureForForward
    default:
      return false
  }
}

/**
 * Insert signature into editor content based on compose mode and placement settings
 */
export function insertSignatureIntoContent(
  content: string,
  signatureHtml: string,
  mode: ComposeMode,
  placement: string = 'above'
): string {
  if (placement === 'above') {
    // Look for a citation line ("On DATE, SENDER wrote:") to insert signature above it.
    // Search for "wrote:" followed by optional <br> and closing </p> tag.
    // This works regardless of compose mode or whether TipTap preserves blockquotes.
    const wroteMatch = content.match(/wrote:\s*(<br[^>]*>)?\s*<\/p>/i)
    if (wroteMatch && wroteMatch.index !== undefined) {
      const before = content.substring(0, wroteMatch.index)
      const pStart = before.lastIndexOf('<p')
      if (pStart > -1) {
        const quotedContent = content.substring(pStart)
        // typed text + blank line below content + signature + 2 blank lines before citation
        return typedContentBeforeSignature(content.substring(0, pStart)) + signatureHtml + '<p></p><p></p>' + quotedContent
      }
    }

    // Check for forwarded message header
    const fwdMatch = content.match(/---------- Forwarded message ----------/)
    if (fwdMatch && fwdMatch.index !== undefined) {
      const fwdBefore = content.substring(0, fwdMatch.index)
      const fwdPStart = fwdBefore.lastIndexOf('<p')
      if (fwdPStart > -1) {
        const forwardedContent = content.substring(fwdPStart)
        return typedContentBeforeSignature(content.substring(0, fwdPStart)) + signatureHtml + '<p></p><p></p>' + forwardedContent
      }
    }

    // Fallback: try blockquote
    const blockquoteIndex = content.indexOf('<blockquote')
    if (blockquoteIndex > -1) {
      const blockquote = content.substring(blockquoteIndex)
      // typed text + blank line below content + signature + 2 blank lines before citation
      return typedContentBeforeSignature(content.substring(0, blockquoteIndex)) + signatureHtml + '<p></p><p></p>' + blockquote
    }
  }

  // New message or no quoted content found
  const isEmpty = content === '<p></p>' || content === '' || content === '<p><br></p>'
  if (isEmpty) {
    // typing area + blank line below content + signature
    return '<p></p><p></p>' + signatureHtml
  }
  return content + '<p></p>' + signatureHtml
}

/**
 * Remove signature from content using the marker, preserving quoted content (replies/forwards).
 * Uses the first marker, or the last one with fromEnd (for a signature placed below quoted text).
 */
export function removeSignatureFromContent(content: string, fromEnd: boolean = false): string {
  const markerIndex = fromEnd ? content.lastIndexOf(SIGNATURE_MARKER) : content.indexOf(SIGNATURE_MARKER)
  if (markerIndex === -1) return content

  // Find the start of the element containing the marker (the <p> tag)
  const beforeMarker = content.substring(0, markerIndex)
  const sigElementStart = beforeMarker.lastIndexOf('<p')
  if (sigElementStart === -1) {
    // Fallback: remove marker to end
    return content.substring(0, markerIndex).replace(/(<br\s*\/?>)+\s*$/, '')
  }

  const afterMarker = content.substring(markerIndex)

  // Look for quoted content after the signature marker:
  // 1. Citation line ("wrote:" pattern)
  const wroteMatch = afterMarker.match(/<p[^>]*>(?:(?!<\/p>).)*wrote:\s*(<br[^>]*>)?\s*<\/p>/i)
  // 2. Blockquote
  const blockquoteMatch = afterMarker.match(/<blockquote/)

  // Find the earliest quoted content boundary
  let quotedStart = -1
  if (wroteMatch?.index !== undefined) {
    quotedStart = markerIndex + wroteMatch.index
  }
  if (blockquoteMatch?.index !== undefined) {
    const bqStart = markerIndex + blockquoteMatch.index
    if (quotedStart === -1 || bqStart < quotedStart) {
      quotedStart = bqStart
    }
  }

  if (quotedStart === -1) {
    // No quoted content found - remove from signature start to end
    let result = content.substring(0, sigElementStart)
    result = result.replace(/(<p>\s*<\/p>\s*)+$/, '')
    result = result.replace(/(<br\s*\/?>)+\s*$/, '')
    return result
  }

  // Preserve quoted content, remove only the signature between
  const beforeSig = content.substring(0, sigElementStart)
  const quotedContent = content.substring(quotedStart)

  return beforeSig + quotedContent
}

/**
 * Check if content already contains a signature marker
 */
export function hasSignatureMarker(content: string): boolean {
  return content.includes(SIGNATURE_MARKER)
}
