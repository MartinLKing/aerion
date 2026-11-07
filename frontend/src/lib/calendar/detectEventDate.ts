// Finds the first upcoming date (and time, if given) mentioned in an email, so
// "Add to calendar" can prefill the event. Handles English, French, German and
// Spanish wording. It's a best guess: the user always reviews the result.
//
// Numeric dates are read day first (03/04 = 3 April), as in Europe, unless the
// second number can't be a month (03/14/2026 is read as March 14). Numeric
// dates need a year, so version numbers and fractions aren't mistaken for
// dates. Relative words ("tomorrow", "next Tuesday") aren't understood.

export interface DetectedDate {
  year: number
  /** 1-12 */
  month: number
  day: number
  /** Absent when the email gives a date but no time */
  hour?: number
  minute?: number
  endHour?: number
  endMinute?: number
  /** True when no upcoming date was found and this one has already passed */
  isPast?: boolean
}

const MONTHS: Record<string, number> = {}
function addMonths(month: number, ...names: string[]) {
  for (const name of names) MONTHS[name] = month
}
// Full names and common abbreviations. Accented and unaccented spellings both
// work, because text is compared after removing accents.
addMonths(1, 'january', 'jan', 'janvier', 'janv', 'januar', 'janner', 'enero', 'ene')
addMonths(2, 'february', 'feb', 'fevrier', 'fevr', 'fev', 'februar', 'febrero')
addMonths(3, 'march', 'mars', 'marz', 'maerz', 'marzo')
addMonths(4, 'april', 'apr', 'avril', 'avr', 'abril', 'abr')
addMonths(5, 'may', 'mai', 'mayo')
addMonths(6, 'june', 'jun', 'juin', 'juni', 'junio')
addMonths(7, 'july', 'jul', 'juillet', 'juil', 'juli', 'julio')
addMonths(8, 'august', 'aug', 'aout', 'agosto', 'ago')
addMonths(9, 'september', 'sep', 'sept', 'septembre', 'septiembre', 'setiembre')
addMonths(10, 'october', 'oct', 'octobre', 'oktober', 'okt', 'octubre')
addMonths(11, 'november', 'nov', 'novembre', 'noviembre')
addMonths(12, 'december', 'dec', 'decembre', 'dezember', 'dez', 'diciembre', 'dic')

// Longest first, so "juillet" wins over "jul"
const MONTH_PATTERN = Object.keys(MONTHS).sort((a, b) => b.length - a.length).join('|')

/** Lowercase and strip accents, so "Décembre" matches "decembre". */
function normalize(text: string): string {
  return text.toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '')
}

function daysInMonth(year: number, month: number): number {
  return new Date(year, month, 0).getDate()
}

function validDate(year: number, month: number, day: number): boolean {
  return month >= 1 && month <= 12 && day >= 1 && day <= daysInMonth(year, month)
}

function expandYear(y: number): number {
  return y < 100 ? 2000 + y : y
}

// monthWord is set for dates written with a month name, at monthIndex
type Candidate = { index: number; end: number; year?: number; month: number; day: number; monthWord?: string; monthIndex?: number }

function findDates(text: string): Candidate[] {
  const out: Candidate[] = []
  let m: RegExpExecArray | null

  // 2026-11-12
  const iso = /\b(\d{4})-(\d{1,2})-(\d{1,2})\b/g
  while ((m = iso.exec(text))) {
    out.push({ index: m.index, end: m.index + m[0].length, year: +m[1], month: +m[2], day: +m[3] })
  }

  // 12/11/2026, 12.11.26, 12-11-2026 (day first unless that's impossible)
  const numeric = /\b(\d{1,2})[./-](\d{1,2})[./-](\d{4}|\d{2})\b/g
  while ((m = numeric.exec(text))) {
    let day = +m[1]
    let month = +m[2]
    if (month > 12 && day <= 12) [day, month] = [month, day]
    out.push({ index: m.index, end: m.index + m[0].length, year: expandYear(+m[3]), month, day })
  }

  // 12 November 2026, 1er décembre, 12. November, 12 de noviembre de 2026
  const dayMonth = new RegExp(
    `\\b(\\d{1,2})(?:er|st|nd|rd|th|\\.)?\\s+(?:de\\s+)?(${MONTH_PATTERN})\\.?(?:,?\\s+(?:de\\s+)?(\\d{4}))?\\b`,
    'g',
  )
  while ((m = dayMonth.exec(text))) {
    out.push({
      index: m.index, end: m.index + m[0].length, year: m[3] ? +m[3] : undefined, month: MONTHS[m[2]], day: +m[1],
      monthWord: m[2], monthIndex: m.index + m[0].indexOf(m[2], m[1].length),
    })
  }

  // November 12, 2026 / Nov 12th. The day can't be the start of a time
  // ("November\n10:15") and must be on the same line as the month.
  const monthDay = new RegExp(
    `\\b(${MONTH_PATTERN})\\.?[^\\S\\n]+(\\d{1,2})(?:st|nd|rd|th)?\\b(?![:.h]\\d)(?:,?\\s+(\\d{4})\\b)?`,
    'g',
  )
  while ((m = monthDay.exec(text))) {
    out.push({
      index: m.index, end: m.index + m[0].length, year: m[3] ? +m[3] : undefined, month: MONTHS[m[1]], day: +m[2],
      monthWord: m[1], monthIndex: m.index,
    })
  }

  // Drop matches overlapping an earlier one: in "06 November 10:15" the month
  // belongs to the first date, not to a second "November 10"
  const sorted = out.sort((a, b) => a.index - b.index)
  const kept: Candidate[] = []
  for (const c of sorted) {
    const last = kept[kept.length - 1]
    if (last && c.index < last.end) continue
    kept.push(c)
  }
  return kept
}

type Time = { hour: number; minute: number; length: number }

// 14:30, 14h30, 14h, 14 h 30, 2:30 pm, 2pm, 14 Uhr, 14.30 Uhr
const TIME_RE = /(\d{1,2})(?:\s*(?::|h|\.(?=\d{2}\s*uhr))\s*(\d{2})?)?\s*(a\.?m\.?|p\.?m\.?|uhr)?(?![\d/])/y

function parseTimeAt(text: string, pos: number): Time | null {
  TIME_RE.lastIndex = pos
  const m = TIME_RE.exec(text)
  if (!m) return null
  const raw = m[0]
  const hasSeparator = /[:h.]/.test(raw.replace(/a\.?m\.?|p\.?m\.?/, ''))
  const suffix = m[3]?.replace(/\./g, '')
  // A bare number isn't a time
  if (!hasSeparator && !suffix) return null
  let hour = +m[1]
  const minute = m[2] ? +m[2] : 0
  if (suffix === 'pm' && hour < 12) hour += 12
  if (suffix === 'am' && hour === 12) hour = 0
  if (hour > 23 || minute > 59) return null
  return { hour, minute, length: raw.trimEnd().length }
}

/** First time in text[from, to), with an optional end time right after it. */
function findTime(text: string, from: number, to: number): Omit<DetectedDate, 'year' | 'month' | 'day'> | null {
  const segment = text.slice(from, to)

  // "von 14 bis 16 Uhr" / "de 14 à 16h": a range whose first number is bare,
  // checked first because the bare "14" alone isn't recognised as a time
  const bareRange = /(?:^|\D)(\d{1,2})\s*(?:-|–|bis|a|à|to)\s*(\d{1,2})\s*(uhr|h)\b/.exec(segment)
  if (bareRange && +bareRange[1] <= 23 && +bareRange[2] <= 23) {
    return { hour: +bareRange[1], minute: 0, endHour: +bareRange[2], endMinute: 0 }
  }

  const start = /\d/g
  let m: RegExpExecArray | null
  while ((m = start.exec(segment))) {
    // Only start at the beginning of a number
    if (m.index > 0 && /\d/.test(segment[m.index - 1])) continue
    const time = parseTimeAt(text, from + m.index)
    if (!time) continue

    const result: Omit<DetectedDate, 'year' | 'month' | 'day'> = { hour: time.hour, minute: time.minute }
    const after = text.slice(from + m.index + time.length)
    const range = /^\s*(?:-|–|to|until|a|à|bis|hasta|jusqu'a)\s*/.exec(after)
    if (range) {
      const end = parseTimeAt(text, from + m.index + time.length + range[0].length)
      // "14-16 Uhr": the first time has no suffix of its own
      if (end) {
        result.endHour = end.hour
        result.endMinute = end.minute
      }
    }
    return result
  }
  return null
}

// A date without a year that passed this recently is probably about the past
// ("sent on 3 November"), not next year
const RECENT_PAST_DAYS = 60

/**
 * Returns the first date in the text that is today or later, with a time if one
 * appears close to it. If every date has passed, returns the first one marked
 * isPast, since adding a past appointment is still useful. A date without a year
 * is taken as this year if it passed recently, otherwise as next year.
 */
export function detectEventDate(text: string, now: Date = new Date()): DetectedDate | null {
  if (!text) return null
  const normalized = normalize(text)
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  let firstPast: DetectedDate | null = null

  for (const c of findDates(normalized)) {
    // English "may" is usually the verb unless capitalised ("you may 5 times")
    if (c.monthWord === 'may' && c.monthIndex !== undefined && text[c.monthIndex] !== 'M') continue

    let year = c.year ?? now.getFullYear()
    if (!validDate(year, c.month, c.day)) continue
    let date = new Date(year, c.month - 1, c.day)
    if (c.year === undefined && date < today) {
      const daysAgo = (today.getTime() - date.getTime()) / 86_400_000
      if (daysAgo > RECENT_PAST_DAYS && validDate(year + 1, c.month, c.day)) {
        year += 1
        date = new Date(year, c.month - 1, c.day)
      }
    }

    // A time is looked for just after the date, then just before it
    const time =
      findTime(normalized, c.end, Math.min(normalized.length, c.end + 40)) ??
      findTime(normalized, Math.max(0, c.index - 25), c.index)
    const found: DetectedDate = { year, month: c.month, day: c.day, ...(time ?? {}) }

    if (date >= today) return found
    firstPast ??= { ...found, isPast: true }
  }
  return firstPast
}
