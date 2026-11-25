// Reads an event's details from the labelled lines of an email ("Check-in ...",
// "Start: ...", "Ort: ...") for "Add to calendar" (Emailio). Understands English,
// German, French and Spanish labels. Falls back to the first date in the text.
// Everything happens locally, and the user reviews the result in the event form.

import { detectDate, detectEventDate, detectTime, type DetectedDate } from './detectEventDate.ts'

export interface ClockTime {
  hour: number
  minute: number
}

export interface ExtractedEvent {
  /** stay: all-day from first to last day; timed: start and maybe end; day: all-day */
  kind: 'stay' | 'timed' | 'day'
  /** False when only a date was found somewhere in the text */
  labelled: boolean
  start: { year: number; month: number; day: number; time?: ClockTime }
  /** For a stay, the last day. Absent when the email gives no end. */
  end?: { year: number; month: number; day: number; time?: ClockTime }
  isPast: boolean
  title: string
  location: string
  /** Short facts for the description, such as "Check-in: ..." */
  summary: string[]
}

// Labels, without accents. Longer spellings first.
const LABELS = {
  checkIn: ['check-in date', 'check-in', 'check in', 'checkin', 'anreise', 'arrival', 'arrivee', 'llegada', 'entrada'],
  checkOut: ['check-out date', 'check-out', 'check out', 'checkout', 'abreise', 'departure', 'depart', 'salida'],
  start: ['start date', 'start time', 'starts', 'start', 'beginn', 'debut', 'inicio', 'comienzo'],
  end: ['end date', 'end time', 'ends', 'end', 'ende', 'fin'],
  from: ['from', 'von', 'de', 'desde'],
  to: ['to', 'nach', 'bis', 'a', 'hasta'],
  date: ['date and time', 'date et heure', 'datum und uhrzeit', 'fecha y hora', 'datum', 'date', 'ihr termin', 'termin', 'appointment', 'when', 'wann', 'quand', 'cuando', 'fecha'],
  time: ['uhrzeit', 'time', 'zeit', 'heure', 'hora'],
  duration: ['duration', 'dauer', 'duree', 'duracion'],
  location: ['veranstaltungsort', 'terminort', 'treffpunkt', 'location', 'address', 'anschrift', 'adresse', 'standort', 'venue', 'where', 'lieu', 'ort', 'wo', 'ubicacion', 'direccion', 'lugar'],
  property: ['property name', 'hotel name', 'property', 'hotel', 'unterkunft', 'etablissement', 'alojamiento'],
  reservation: ['your reservation', 'ihre reservierung', 'votre reservation', 'su reserva', 'room type', 'zimmer'],
  guests: ['you booked for', 'number of guests', 'guests', 'gaste', 'personnes', 'huespedes'],
  phone: ['phone number', 'telephone', 'telefono', 'telefon', 'phone', 'tel'],
  number: ['confirmation number', 'booking number', 'reservation number', 'order number', 'buchungsnummer', 'buchungscode', 'reservierungsnummer', 'auftragsnummer', 'numero de reservation', 'numero de reserva', 'confirmation'],
} as const

type Field = keyof typeof LABELS

// The header lines of a quoted or forwarded email also say From, To and Date
const MAIL_HEADER = /^[>\s]*(from|to|cc|sent|subject|von|an|gesendet|betreff|de|a|objet|envoye|para|asunto|enviado)\s*:/

function plain(text: string): string {
  return text.toLowerCase().normalize('NFD').replace(/[\u0300-\u036f]/g, '')
}

function toLines(text: string): string[] {
  return text
    .replace(/\r\n?/g, '\n')
    .replace(/\u200b|\u200c|\u200d|\ufeff/g, '')
    .split('\n')
    .map((l) => l.replace(/\u00a0/g, ' ').trim())
}

interface Labelled {
  line: number
  /** Text after the label on its own line */
  value: string
  colon: boolean
}

/** Splits "Check-in Thursday" into its label and value, if it starts with a label. */
function matchLabel(line: string, field: Field): { value: string; colon: boolean } | null {
  const bare = line.replace(/^[*•\-–|>\s]+/, '')
  const p = plain(bare)
  // Removing accents can change the length; then the value can't be cut out safely
  const source = p.length === bare.length ? bare : p
  for (const label of LABELS[field]) {
    if (!p.startsWith(label)) continue
    const rest = source.slice(label.length)
    const colon = /^\s*:/.test(rest)
    // Either a real boundary, or "DateFriday" / "Time10:15" glued together
    if (!colon && rest !== '' && !/^[\s(–-]/.test(rest) && !/^[A-Z0-9]/.test(rest)) continue
    return { value: rest.replace(/^\s*[:–-]?\s*/, '').trim(), colon }
  }
  return null
}

function isAnyLabel(line: string): boolean {
  return (Object.keys(LABELS) as Field[]).some((f) => {
    if (f === 'from' || f === 'to') return false
    const m = matchLabel(line, f)
    return !!m && (m.colon || m.value === '')
  })
}

function inMailHeader(lines: string[], index: number): boolean {
  let headers = 0
  for (let i = Math.max(0, index - 4); i <= Math.min(lines.length - 1, index + 4); i++) {
    if (MAIL_HEADER.test(plain(lines[i]))) headers++
  }
  return headers >= 2
}

function findLabelled(lines: string[], field: Field): Labelled[] {
  const out: Labelled[] = []
  lines.forEach((line, i) => {
    const m = matchLabel(line, field)
    if (m) out.push({ line: i, ...m })
  })
  return out
}

/** The next non-empty lines after a label, up to the next label. */
function following(lines: string[], from: number, max: number): string[] {
  const out: string[] = []
  for (let i = from + 1; i < lines.length && i <= from + max + 4 && out.length < max; i++) {
    if (lines[i] === '') {
      // Blank lines right after the label are layout; later ones end the value
      if (out.length > 0) break
      continue
    }
    if (isAnyLabel(lines[i])) break
    out.push(lines[i])
  }
  return out
}

interface DateHit extends Labelled {
  date: DetectedDate
  /** The text the date was read from */
  text: string
  /** Text between the label and the date, such as a station name */
  before: string
}

function findDate(lines: string[], field: Field, now: Date, spread = 2): DateHit | null {
  for (const hit of findLabelled(lines, field)) {
    if (inMailHeader(lines, hit.line)) continue

    if (hit.value) {
      // Without a colon it may be a sentence: the date must follow at once
      const scope = hit.colon ? hit.value : hit.value.slice(0, 60)
      const found = detectDate(scope, now)
      if (found && (hit.colon || found.index <= 14)) {
        return { ...hit, date: found.date, text: hit.value, before: scope.slice(0, found.index) }
      }
      if (!hit.colon) continue
    }

    const next = following(lines, hit.line, spread)
    const text = [hit.value, ...next].filter(Boolean).join('\n')
    const found = detectDate(text, now)
    if (found) return { ...hit, date: found.date, text: text.replace(/\n/g, ' '), before: text.slice(0, found.index) }
  }
  return null
}

function findText(lines: string[], field: Field, maxLines = 1): string {
  for (const hit of findLabelled(lines, field)) {
    if (hit.value) {
      if (hit.colon) return hit.value
      continue
    }
    const next = following(lines, hit.line, maxLines)
    if (next.length) return next.join(' ').replace(/,\s*,/g, ',')
  }
  return ''
}

/** A short value on the label's own line, with or without a colon. */
function findShort(lines: string[], field: Field, test: RegExp): string {
  for (const hit of findLabelled(lines, field)) {
    const value = hit.value || following(lines, hit.line, 1)[0] || ''
    if (value.length <= 90 && test.test(value)) return value
  }
  return ''
}

function clock(d: { hour?: number; minute?: number }): ClockTime | undefined {
  return d.hour === undefined ? undefined : { hour: d.hour, minute: d.minute ?? 0 }
}

function dayValue(d: { year: number; month: number; day: number }): number {
  return Date.UTC(d.year, d.month - 1, d.day)
}

function ymd(d: DetectedDate) {
  return { year: d.year, month: d.month, day: d.day }
}

/** Moves a point in time by minutes, across days if needed. */
function addMinutes(d: { year: number; month: number; day: number }, t: ClockTime, minutes: number) {
  const x = new Date(Date.UTC(d.year, d.month - 1, d.day, t.hour, t.minute + minutes))
  return {
    year: x.getUTCFullYear(), month: x.getUTCMonth() + 1, day: x.getUTCDate(),
    time: { hour: x.getUTCHours(), minute: x.getUTCMinutes() },
  }
}

function durationMinutes(text: string): number {
  const p = plain(text)
  const h = /(\d+)(?:[.,](\d+))?\s*(?:hours?|hrs?|stunden?|std|heures?|horas?|h)\b/.exec(p)
  const m = /(\d+)\s*(?:minutes?|minuten?|minutos?|min)\b/.exec(p)
  let total = 0
  if (h) total += +h[1] * 60 + (h[2] ? Math.round(+`0.${h[2]}` * 60) : 0)
  if (m) total += +m[1]
  return total > 0 && total <= 24 * 60 ? total : 0
}

function cleanSubject(subject: string): string {
  return subject
    .replace(/^\s*((re|aw|fwd?|wg|tr|rv)\s*:\s*)+/i, '')
    .replace(/^[^\p{L}\p{N}"'([]+/u, '')
    .trim()
}

function stayTitle(lines: string[], subject: string): string {
  const named = findLabelled(lines, 'property').find((h) => h.colon && h.value && h.value.length <= 80)
  if (named) return named.value
  for (const line of lines) {
    const m = /^(?:\[[^\]]*\]\s*)*(.{3,80}?) (?:is expecting you|erwartet sie|vous attend|le espera)\b/i.exec(line)
    if (m) return m[1]
  }
  const at = /(?:\bat|@|\bbei|\bchez)\s+(\S.{2,80})$/i.exec(cleanSubject(subject))
  return at ? at[1].trim() : cleanSubject(subject)
}

/**
 * Removes what doesn't belong in an event description: image placeholders,
 * links, rules, and PINs or passwords, as calendars are often shared.
 */
export function cleanEmailText(text: string, options: { keepLinks?: boolean } = {}): string {
  const out: string[] = []
  let skipNext = false
  for (const raw of toLines(text)) {
    if (skipNext && raw !== '') {
      skipNext = false
      continue
    }
    if (isSecretLine(raw)) {
      skipNext = /^\W*[\p{L} -]+\W*$/u.test(raw) && raw.length < 25
      if (skipNext || /\d|:/.test(raw)) continue
    }
    const unlinked = options.keepLinks ? raw : raw
      .replace(/[(<]\s*https?:\/\/[^\s)>]*\s*[)>]?/gi, '')
      .replace(/https?:\/\/\S+/gi, '')
    const line = unlinked
      .replace(/\[[^\]\n]*\.(?:png|jpe?g|gif|svg|webp)\]/gi, '')
      .replace(/[_*=-]{4,}/g, '')
      .replace(/\s{2,}/g, ' ')
      .trim()
    if (/^[()<>|\s]*$/.test(line) && out[out.length - 1] === '') continue
    out.push(/^[()<>|\s]*$/.test(line) ? '' : line)
  }
  return out.join('\n').replace(/\n{3,}/g, '\n\n').trim()
}

function isSecretLine(line: string): boolean {
  return /\b(?:pin|pin[- ]?code|passwor[dt]|passcode|kennwort|mot de passe|contrasena)\b/.test(plain(line)) &&
    (/\d/.test(line) || /:/.test(line) || line.length < 25)
}

export function extractEvent(subject: string, text: string, now: Date = new Date()): ExtractedEvent | null {
  const lines = toLines(text).map((l) => (isSecretLine(l) ? '' : l.replace(/^(?:\[[^\]]*\.\w{3,4}\]\s*)+/, '')))
  const today = Date.UTC(now.getFullYear(), now.getMonth(), now.getDate())

  // Google marks changed details of an invitation this way
  const location = findText(lines, 'location', 4).replace(/^CHANGED\s+/, '').slice(0, 250)
  const number = findShort(lines, 'number', /^[A-Z0-9][A-Z0-9-]{3,20}$/i)
  const phone = findShort(lines, 'phone', /^\+?[\d(][\d\s()/.-]{5,24}$/)
  const common = [number && `Booking number: ${number}`, phone && `Phone: ${phone}`].filter(Boolean) as string[]

  // Overnight stay: check-in and a later check-out
  const checkIn = findDate(lines, 'checkIn', now)
  const checkOut = checkIn && findDate(lines, 'checkOut', now)
  if (checkIn && checkOut) {
    const nights = (dayValue(checkOut.date) - dayValue(checkIn.date)) / 86_400_000
    if (nights >= 1 && nights <= 90) {
      const reservation = findShort(lines, 'reservation', /\S/)
      const guests = findShort(lines, 'guests', /\d/)
      return {
        kind: 'stay', labelled: true,
        start: ymd(checkIn.date), end: ymd(checkOut.date),
        isPast: dayValue(checkOut.date) < today,
        title: stayTitle(lines, subject),
        location,
        summary: [
          `Check-in: ${checkIn.text}`, `Check-out: ${checkOut.text}`,
          ...(reservation ? [reservation] : []), ...(guests ? [`Guests: ${guests}`] : []), ...common,
        ],
      }
    }
  }

  const build = (startHit: DateHit, endDate: ExtractedEvent['end'], title: string, place: string): ExtractedEvent => {
    const time = clock(startHit.date)
    let end = endDate
    if (time && !end) {
      const minutes = durationMinutes(findShort(lines, 'duration', /\d/))
      if (startHit.date.endHour !== undefined) {
        end = { ...ymd(startHit.date), time: { hour: startHit.date.endHour, minute: startHit.date.endMinute ?? 0 } }
      } else if (minutes) {
        end = addMinutes(ymd(startHit.date), time, minutes)
      }
    }
    // Past midnight
    if (time && end?.time && dayValue(end) === dayValue(startHit.date) &&
        end.time.hour * 60 + end.time.minute <= time.hour * 60 + time.minute) {
      end = addMinutes(end, end.time, 24 * 60)
    }
    return {
      kind: time ? 'timed' : 'day', labelled: true,
      start: { ...ymd(startHit.date), time }, end,
      isPast: dayValue(startHit.date) < today,
      title, location: place, summary: common,
    }
  }

  // Start and end
  const start = findDate(lines, 'start', now)
  if (start) {
    const endHit = findDate(lines, 'end', now)
    let end: ExtractedEvent['end']
    if (endHit && dayValue(endHit.date) >= dayValue(start.date)) {
      end = { ...ymd(endHit.date), time: clock(endHit.date) }
    } else {
      // "Ende: 20:00 Uhr"
      const endText = findShort(lines, 'end', /\d/)
      const t = endText ? detectTime(endText) : null
      if (t) end = { ...ymd(start.date), time: t }
    }
    // The line above "Start:" is often the name of what was booked
    const above = lines[start.line - 1] ?? ''
    const named = above.length >= 3 && above.length <= 60 && !/[.:!?,;]$/.test(above) && !isAnyLabel(above)
    return build(start, end, named ? above : cleanSubject(subject), location)
  }

  // Journey: from a place and time to another
  const from = findDate(lines, 'from', now)
  const to = from && findDate(lines, 'to', now)
  if (from && to && from.date.hour !== undefined && to.date.hour !== undefined && to.line > from.line) {
    const hours = (dayValue(to.date) - dayValue(from.date)) / 3_600_000 +
      (to.date.hour - from.date.hour) + ((to.date.minute ?? 0) - (from.date.minute ?? 0)) / 60
    if (hours > 0 && hours <= 48) {
      const name = (s: string) => s.replace(/\s+/g, ' ').replace(/[\s,(]+$/, '').trim()
      const a = name(from.before)
      const b = name(to.before)
      const title = a && b && a.length <= 60 && b.length <= 60 ? `${a} → ${b}` : cleanSubject(subject)
      return build(from, { ...ymd(to.date), time: clock(to.date) }, title, a.length <= 60 ? a : '')
    }
  }

  // A labelled date, with the time on a line of its own
  const dated = findDate(lines, 'date', now)
  if (dated) {
    if (dated.date.hour === undefined) {
      const timeText = findShort(lines, 'time', /\d/)
      const t = timeText ? detectTime(timeText) : null
      if (t) {
        dated.date = { ...dated.date, ...t }
      }
    }
    return build(dated, undefined, cleanSubject(subject), location)
  }

  // Nothing labelled: the first upcoming date in the subject, then the text
  const fromSubject = detectEventDate(subject, now)
  const fromText = !fromSubject || fromSubject.isPast ? detectEventDate(lines.join('\n'), now) : null
  const guess = fromText && !fromText.isPast ? fromText : (fromSubject ?? fromText)
  if (!guess) return null
  const event = build({ line: 0, value: '', colon: false, date: guess, text: '', before: '' }, undefined, cleanSubject(subject), location)
  return { ...event, labelled: false }
}
