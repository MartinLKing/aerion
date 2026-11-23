// Which attachments a message's HTML body displays by itself (Emailio).

interface InlineCandidate {
  contentType: string
  contentId?: string
  isInline: boolean
  size: number
}

// Larger inline images are not stored for display (maxInlineContentSize in
// internal/sync), so the body can't show them.
const MAX_INLINE_SIZE = 5 * 1024 * 1024

/** Drops the attachments the body displays; everything else needs a chip. */
export function notShownInBody<T extends InlineCandidate>(attachments: T[], bodyHtml: string): T[] {
  // Same pattern the message body uses to fill in images
  const shown = new Set([...(bodyHtml || '').matchAll(/src=["']cid:([^"']+)["']/gi)].map((m) => m[1]))
  return attachments.filter(
    (att) =>
      !(
        att.isInline &&
        att.contentType.startsWith('image/') &&
        att.size <= MAX_INLINE_SIZE &&
        !!att.contentId &&
        shown.has(att.contentId)
      ),
  )
}
