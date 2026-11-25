<script lang="ts">
  // "Add to calendar" for an email: opens the Calendar extension's event form
  // prefilled from the message: from an attached calendar file if there is one,
  // else from details in the text. The calendar code is loaded only on click, so
  // mail doesn't load it while the Calendar view has never been opened.
  import type { Component } from 'svelte'
  import Icon from '@iconify/svelte'
  import * as Dialog from '$lib/components/ui/dialog'
  import { _ } from '$lib/i18n'
  import { toasts } from '$lib/stores/toast'
  import { dialogGuardOpen, dialogGuardClose } from '$lib/stores/dialogGuard'
  import { cleanEmailText, extractEvent, type ExtractedEvent } from '$lib/calendar/extractEvent'
  import { escapeHTML, icsEmailPrefill, type EventPrefill, type ICSEvent } from '$lib/calendar/icsPrefill'
  import { htmlToLines } from '$lib/calendar/htmlToLines'
  // @ts-ignore - wailsjs path
  import { GetEmailEvents } from '../../../../wailsjs/go/app/App'
  // @ts-ignore - wailsjs path
  import { message as messageModels } from '../../../../wailsjs/go/models'

  interface Props {
    msg: messageModels.Message
  }

  let { msg }: Props = $props()

  // Length of the email text copied into the event description
  const DESCRIPTION_CHARS = 500
  // With this many facts found, the email's text isn't needed
  const ENOUGH_FACTS = 3

  let ComposerDialog = $state<Component<any> | null>(null)
  let prefill = $state<EventPrefill | null>(null)
  let open = $state(false)
  let preparing = $state(false)
  // Several events in the attached files, to choose from
  let choices = $state<ICSEvent[]>([])
  let choosing = $state(false)

  $effect(() => {
    if (!choosing) return
    dialogGuardOpen()
    return () => dialogGuardClose()
  })

  function emailText(): string {
    return (msg.bodyText || msg.snippet || '').trim()
  }

  function buildDescription(facts: string[]): string {
    const sender = msg.fromName ? `${msg.fromName} <${msg.fromEmail}>` : msg.fromEmail
    const received = msg.date ? new Date(msg.date as unknown as string).toLocaleString() : ''

    let html = ''
    if (facts.length) html += `<p>${facts.map(escapeHTML).join('<br>')}</p>`
    if (facts.length < ENOUGH_FACTS) {
      let text = cleanEmailText(emailText())
      if (text.length > DESCRIPTION_CHARS) text = text.slice(0, DESCRIPTION_CHARS).trimEnd() + '…'
      if (text) html += `<p>${escapeHTML(text).replace(/\n/g, '<br>')}</p>`
    }
    const lines = [`${$_('addToCalendar.from')}: ${escapeHTML(sender || '')}`]
    if (received) lines.push(`${$_('addToCalendar.received')}: ${escapeHTML(received)}`)
    return html + `<p>${lines.join('<br>')}</p>`
  }

  // Emails whose text part leaves the details out have them in the HTML
  function readEmail(): ExtractedEvent | null {
    const subject = msg.subject || ''
    const fromText = extractEvent(subject, emailText())
    if (fromText?.labelled || !msg.bodyHtml) return fromText
    const fromHtml = extractEvent(subject, htmlToLines(msg.bodyHtml))
    return fromHtml && (fromHtml.labelled || !fromText) ? fromHtml : fromText
  }

  // No connection or an unreadable file: the text is used instead
  async function attachedEvents(): Promise<ICSEvent[]> {
    if (!msg.hasAttachments) return []
    try {
      return ((await GetEmailEvents(msg.id)) as ICSEvent[]) || []
    } catch (err) {
      console.error('Failed to read attached calendar files:', err)
      return []
    }
  }

  function showIcs(ev: ICSEvent) {
    choosing = false
    const notes = [$_('icsOpen.fromFile', { values: { file: ev.fileName } })]
    if (ev.recurring) notes.push($_('icsOpen.recurring'))
    if (ev.startUnix * 1000 < Date.now()) notes.push($_('addToCalendar.alreadyPast'))
    prefill = icsEmailPrefill(ev, $_('icsOpen.organizer'), notes.join(' '))
    if (!prefill.summary) prefill.summary = msg.subject || ''
    open = true
  }

  function showText(found: ExtractedEvent | null, toDate: (y: number, mo: number, d: number, h: number, mi: number) => Date) {
    const date = (p: NonNullable<ExtractedEvent['end']>) => toDate(p.year, p.month, p.day, p.time?.hour ?? 0, p.time?.minute ?? 0)
    const allDay = !!found && found.kind !== 'timed'
    prefill = {
      start: found ? date(found.start) : null,
      // A timed event needs an end with a time; otherwise the form's default
      end: found?.end && (allDay || found.end.time) ? date(found.end) : null,
      allDay,
      summary: found?.title || msg.subject || '',
      location: found?.location || '',
      descriptionHTML: buildDescription(found?.summary ?? []),
      notice: !found
        ? $_('addToCalendar.noDateFound')
        : found.isPast ? $_('addToCalendar.pastDateFound')
          : found.labelled ? $_('addToCalendar.detailsFound') : $_('addToCalendar.dateFound'),
    }
    open = true
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
      ComposerDialog = dialog as unknown as Component<any>

      const events = await attachedEvents()
      if (events.length > 1) {
        choices = [...events].sort((a, b) => a.startUnix - b.startUnix)
        choosing = true
        return
      }
      if (events.length === 1) {
        showIcs(events[0])
        return
      }

      // Build times in the calendar's timezone, which may differ from the computer's
      const tz = calendarSettings.effectiveTimezone
      showText(readEmail(), (y, mo, d, h, mi) => fromZonedTime(new Date(y, mo - 1, d, h, mi, 0, 0), tz))
    } catch (err) {
      console.error('Failed to open the event form:', err)
      toasts.error($_('addToCalendar.failed'))
    } finally {
      preparing = false
    }
  }

  function when(ev: ICSEvent): string {
    const d = new Date(ev.startUnix * 1000)
    return ev.allDay ? d.toLocaleDateString() : d.toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' })
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

<Dialog.Root bind:open={choosing}>
  <Dialog.Content class="max-w-md">
    <Dialog.Header>
      <Dialog.Title>{$_('addToCalendar.choose')}</Dialog.Title>
    </Dialog.Header>
    <div class="max-h-80 overflow-y-auto space-y-1">
      {#each choices as ev, i (i)}
        <button class="w-full rounded-md px-3 py-2 text-left hover:bg-muted transition-colors" onclick={() => showIcs(ev)}>
          <span class="block text-sm font-medium truncate">{ev.summary || ev.fileName}</span>
          <span class="block text-xs text-muted-foreground truncate">{when(ev)}{ev.location ? ` · ${ev.location}` : ''}</span>
        </button>
      {/each}
    </div>
  </Dialog.Content>
</Dialog.Root>

{#if ComposerDialog && prefill}
  <ComposerDialog
    bind:open
    mode="create"
    defaultStart={prefill.start}
    defaultEnd={prefill.end}
    defaultAllDay={prefill.allDay}
    initialSummary={prefill.summary}
    initialLocation={prefill.location}
    initialDescriptionHTML={prefill.descriptionHTML}
    notice={prefill.notice}
    onClose={() => { open = false }}
    onSaved={() => { open = false }}
  />
{/if}
