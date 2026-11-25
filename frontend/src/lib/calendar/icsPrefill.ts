// Turns an event read from an .ics file into what the event form needs
// (Emailio). Shared by opening .ics files and "Add to calendar".

import { cleanEmailText } from './extractEvent.ts'

export type ICSEvent = {
  fileName: string
  summary: string
  location: string
  description: string
  startUnix: number
  endUnix: number
  allDay: boolean
  organizerName: string
  organizerEmail: string
  recurring: boolean
  eventCount: number
}

export type EventPrefill = {
  start: Date | null
  end: Date | null
  allDay: boolean
  summary: string
  location: string
  descriptionHTML: string
  notice: string
}

const DAY_MS = 24 * 60 * 60 * 1000

export function escapeHTML(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
}

export function icsPrefill(ev: ICSEvent, organizerLabel: string, notice: string): EventPrefill {
  const start = new Date(ev.startUnix * 1000)
  let end: Date | null = ev.endUnix > ev.startUnix ? new Date(ev.endUnix * 1000) : null
  // In .ics files an all-day event ends at the start of the next day; the
  // form's end date is the last day of the event.
  if (ev.allDay && end) {
    end = new Date(end.getTime() - DAY_MS)
    if (end <= start) end = null
  }

  let html = ''
  if (ev.organizerEmail || ev.organizerName) {
    const who = ev.organizerName && ev.organizerEmail
      ? `${ev.organizerName} <${ev.organizerEmail}>`
      : ev.organizerName || ev.organizerEmail
    html += `<p>${escapeHTML(organizerLabel)}: ${escapeHTML(who)}</p>`
  }
  if (ev.description) html += `<p>${escapeHTML(ev.description).replace(/\r?\n/g, '<br>')}</p>`

  return {
    start, end, allDay: ev.allDay,
    summary: ev.summary, location: ev.location.trim(),
    descriptionHTML: html, notice,
  }
}

/** As icsPrefill, for a file inside an email: PINs are left out. */
export function icsEmailPrefill(ev: ICSEvent, organizerLabel: string, notice: string): EventPrefill {
  // Links stay: in an invitation they are how to join the meeting
  return icsPrefill({ ...ev, description: cleanEmailText(ev.description, { keepLinks: true }) }, organizerLabel, notice)
}
