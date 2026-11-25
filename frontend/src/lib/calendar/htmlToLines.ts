// The text of an HTML email, one line per block or table cell, for reading
// event details (Emailio). The HTML is parsed without being shown or loaded.

const BREAKS = 'p,div,br,tr,td,th,li,ul,ol,table,h1,h2,h3,h4,h5,h6,section,article,header,footer,blockquote,hr,dt,dd'

export function htmlToLines(html: string): string {
  const doc = new DOMParser().parseFromString(html, 'text/html')
  doc.querySelectorAll('style,script,head,title').forEach((el) => el.remove())
  // Line breaks in the source are only spaces in HTML
  const walker = doc.createTreeWalker(doc.body, NodeFilter.SHOW_TEXT)
  while (walker.nextNode()) {
    const node = walker.currentNode
    node.textContent = (node.textContent || '').replace(/\s+/g, ' ')
  }
  doc.body.querySelectorAll(BREAKS).forEach((el) => {
    el.before('\n')
    el.after('\n')
  })
  return (doc.body.textContent || '').replace(/[ \t]*\n[ \t]*/g, '\n').replace(/\n{3,}/g, '\n\n').trim()
}
