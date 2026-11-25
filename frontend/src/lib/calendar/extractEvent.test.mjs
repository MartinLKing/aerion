// Run with: npm run test:calendar (Node's own test runner, no extra packages)
import test from 'node:test'
import assert from 'node:assert/strict'
import { extractEvent, cleanEmailText } from './extractEvent.ts'

const now = new Date(2026, 10, 25, 9, 0)
const at = (d) => d && `${d.year}-${d.month}-${d.day}${d.time ? ` ${d.time.hour}:${String(d.time.minute).padStart(2, '0')}` : ''}`

const HOTEL = `
   Booking.com
   Confirmation: 1234567890
   PIN: 4321 (Confidential)

   [checkmark.png] The Example Hotel is expecting you on Thu, Nov 26
   [lock_closed.png] Keep your PIN confidential as it can be used to
   modify or cancel your booking.

Reservation details

   Check-in Thursday, November 26, 2026 (3:00 PM - 12:00 AM)
   Check-out Friday, November 27, 2026 (12:00 AM - 11:00 AM)
   Your reservation 1 night, Triple Room with Private Bathroom
   You booked for 2 adults, 1 child (17 years old)
   Location


    Example Street 11, Amsterdam City Center, 1012 AG Amsterdam,
    Netherlands

   Phone +31201234567
   Contact Email property
   Cancellation cost
     * from November 24, 2026 10:38 PM: € 80.50
`

test('hotel stay', () => {
  const e = extractEvent('🛄 Thanks! Your booking is confirmed at The Example Hotel', HOTEL, now)
  assert.equal(e.kind, 'stay')
  assert.equal(at(e.start), '2026-11-26')
  assert.equal(at(e.end), '2026-11-27')
  assert.equal(e.title, 'The Example Hotel')
  assert.equal(e.location, 'Example Street 11, Amsterdam City Center, 1012 AG Amsterdam, Netherlands')
  assert.deepEqual(e.summary, [
    'Check-in: Thursday, November 26, 2026 (3:00 PM - 12:00 AM)',
    'Check-out: Friday, November 27, 2026 (12:00 AM - 11:00 AM)',
    '1 night, Triple Room with Private Bathroom',
    'Guests: 2 adults, 1 child (17 years old)',
    'Booking number: 1234567890',
    'Phone: +31201234567',
  ])
  assert.equal(e.isPast, false)
})

test('no PIN, placeholders or links in the description', () => {
  const t = cleanEmailText(HOTEL + '\nSee details\n( https://example.com/x?y=1 )\n*****************\nPIN\n9876\nEnd')
  assert.ok(!t.includes('4321'))
  assert.ok(!t.includes('9876'))
  assert.ok(!/\.png|https?:|\*{4}/.test(t))
  assert.ok(t.includes('See details'))
})

test('start and end, with the name above', () => {
  const e = extractEvent('Terminbestätigung', `Lieber Max,
vielen Dank für deine Terminbuchung:
Yoga Club
Start: 11.12.2026, 19:00 Uhr
Ende: 11.12.2026, 20:00 Uhr
Ort: Fitness Bonn, Musterstraße 24, Bonn`, now)
  assert.equal(e.title, 'Yoga Club')
  assert.equal(at(e.start), '2026-12-11 19:00')
  assert.equal(at(e.end), '2026-12-11 20:00')
  assert.equal(e.location, 'Fitness Bonn, Musterstraße 24, Bonn')
})

test('date, time, duration and place on separate lines; past dates are marked', () => {
  const e = extractEvent('Ihre Terminvereinbarung', `Terminort:
Amt Bonn
Musterstr. 101
53123 Bonn

Ihr Termin:
Datum: 21.07.2026
Uhrzeit: 08:50 Uhr
Dauer: ca. 40 Minuten`, now)
  assert.equal(at(e.start), '2026-7-21 8:50')
  assert.equal(at(e.end), '2026-7-21 9:30')
  assert.equal(e.location, 'Amt Bonn Musterstr. 101 53123 Bonn')
  assert.equal(e.isPast, true)
})

test('labels glued to their values', () => {
  const e = extractEvent('Your appointment is confirmed', 'DateFriday 04 December\nTime10:15\nyour practitionerDr. Muster', now)
  assert.equal(at(e.start), '2026-12-4 10:15')
})

test('journey', () => {
  const e = extractEvent('Booking confirmation', `The order was booked on 01.12.2026 at 08:08 as follows:
 from
 Frankfurt Flughafen,
 12.12.2026 08:26
 to
 Siegburg/Bonn,
 12.12.2026 09:17
 Payment of 148,35 EUR`, now)
  assert.equal(e.title, 'Frankfurt Flughafen → Siegburg/Bonn')
  assert.equal(at(e.start), '2026-12-12 8:26')
  assert.equal(at(e.end), '2026-12-12 9:17')
  assert.equal(e.location, 'Frankfurt Flughafen')
})

test('headers of a quoted email are not event details', () => {
  const e = extractEvent('Re: Lunch', `See you on 5 December at 12:30!

From: A <a@example.com>
Date: Tue, 1 December 2026 10:00
To: B <b@example.com>
Subject: Lunch`, now)
  assert.equal(at(e.start), '2026-12-5 12:30')
  assert.equal(e.labelled, false)
})

test('other cases', () => {
  // French stay, label and value in separate table cells
  const fr = extractEvent('Réservation', 'Arrivée\n26 décembre 2026\nDépart\n28 décembre 2026\nAdresse : 1 rue Exemple, Paris', now)
  assert.equal(fr.kind, 'stay')
  assert.equal(at(fr.end), '2026-12-28')
  assert.equal(fr.location, '1 rue Exemple, Paris')
  // A meeting past midnight
  const night = extractEvent('Party', 'When: 31 December 2026, 22:00 - 02:00', now)
  assert.equal(at(night.end), '2027-1-1 2:00')
  // Sentences starting with a label word aren't details
  const prose = extractEvent('News', 'Start your free trial today. Offer valid to 30 November 2026.', now)
  assert.equal(prose.labelled, false)
  assert.equal(extractEvent('Hello', 'Nothing here', now), null)
})
