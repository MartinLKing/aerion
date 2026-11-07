<script lang="ts">
  // "Add to calendar" for an email: opens the Calendar extension's event form
  // prefilled from the message. The calendar code is loaded only on click, so
  // mail doesn't load it while the Calendar view has never been opened.
  import type { Component } from 'svelte'
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import { toasts } from '$lib/stores/toast'
  import { detectEventDate } from '$lib/calendar/detectEventDate'
  // @ts-ignore - wailsjs path
  import { message as messageModels } from '../../../../wailsjs/go/models'

  interface Props {
    msg: messageModels.Message
  }

  let { msg }: Props = $props()

  // Length of the email text copied into the event description
  const DESCRIPTION_CHARS = 500

  type Prefill = {
    start: Date | null
    end: Date | null
    allDay: boolean
    summary: string
    descriptionHTML: string
    notice: string
  }

  let ComposerDialog = $state<Component<any> | null>(null)
  let prefill = $state<Prefill | null>(null)
  let open = $state(false)
  let preparing = $state(false)

  function escapeHTML(s: string): string {
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
  }

  function emailText(): string {
    return (msg.bodyText || msg.snippet || '').replace(/\r\n/g, '\n').trim()
  }

  function buildDescription(): string {
    const sender = msg.fromName ? `${msg.fromName} <${msg.fromEmail}>` : msg.fromEmail
    const received = msg.date ? new Date(msg.date as unknown as string).toLocaleString() : ''
    let text = emailText()
    if (text.length > DESCRIPTION_CHARS) text = text.slice(0, DESCRIPTION_CHARS).trimEnd() + '…'

    const lines = [`${$_('addToCalendar.from')}: ${escapeHTML(sender || '')}`]
    if (received) lines.push(`${$_('addToCalendar.received')}: ${escapeHTML(received)}`)
    let html = `<p>${lines.join('<br>')}</p>`
    if (text) html += `<p>${escapeHTML(text).replace(/\n/g, '<br>')}</p>`
    return html
  }

  async function handleClick() {
    if (preparing) return
    preparing = true
    try {
      const [{ default: dialog }, { calendarSources }, { calendarSettings }, { fromZonedTime }] = await Promise.all([
        import('$extensions/calendar/frontend/components/EventComposerDialog.svelte'),
        import('$extensions/calendar/frontend/stores/calendarSources.svelte'),
        import('$extensions/calendar/frontend/stores/calendarSettings.svelte'),
        import('date-fns-tz'),
      ])

      if (calendarSources.sources.length === 0) await calendarSources.load()
      const hasWritable = calendarSources.sources.some((src: any) =>
        src.writable && (calendarSources.calendarsBySource[src.id] || []).some((c: any) => c.writable !== false),
      )
      if (!hasWritable) {
        toasts.info($_('addToCalendar.noCalendar'))
        return
      }

      // Subject first, then the email text. An upcoming date in the text beats a
      // past one in the subject.
      const fromSubject = detectEventDate(msg.subject || '')
      const fromText = !fromSubject || fromSubject.isPast ? detectEventDate(emailText()) : null
      const found = fromText && !fromText.isPast ? fromText : (fromSubject ?? fromText)
      const tz = calendarSettings.effectiveTimezone
      // Build times in the calendar's timezone, which may differ from the computer's
      const at = (h: number, m: number) =>
        fromZonedTime(new Date(found!.year, found!.month - 1, found!.day, h, m, 0, 0), tz)

      let start: Date | null = null
      let end: Date | null = null
      let allDay = false
      if (found && found.hour !== undefined) {
        start = at(found.hour, found.minute ?? 0)
        if (found.endHour !== undefined) end = at(found.endHour, found.endMinute ?? 0)
      } else if (found) {
        start = at(0, 0)
        allDay = true
      }

      prefill = {
        start,
        end,
        allDay,
        summary: msg.subject || '',
        descriptionHTML: buildDescription(),
        notice: !found
          ? $_('addToCalendar.noDateFound')
          : found.isPast ? $_('addToCalendar.pastDateFound') : $_('addToCalendar.dateFound'),
      }
      ComposerDialog = dialog as unknown as Component<any>
      open = true
    } catch (err) {
      console.error('Failed to open the event form:', err)
      toasts.error($_('addToCalendar.failed'))
    } finally {
      preparing = false
    }
  }
</script>

<button
  class="inline-flex items-center gap-1 text-primary hover:underline disabled:opacity-50"
  onclick={handleClick}
  disabled={preparing}
>
  <Icon icon={preparing ? 'mdi:loading' : 'mdi:calendar-plus'} class="w-4 h-4 {preparing ? 'animate-spin' : ''}" />
  {$_('addToCalendar.button')}
</button>

{#if ComposerDialog && prefill}
  <ComposerDialog
    bind:open
    mode="create"
    defaultStart={prefill.start}
    defaultEnd={prefill.end}
    defaultAllDay={prefill.allDay}
    initialSummary={prefill.summary}
    initialDescriptionHTML={prefill.descriptionHTML}
    notice={prefill.notice}
    onClose={() => { open = false }}
    onSaved={() => { open = false }}
  />
{/if}
