<script lang="ts">
  // Opening .ics files (Emailio addition). When Emailio is started with an
  // .ics file, or handed one while running, the event is shown in the
  // calendar's new-event form to be added to a calendar. The calendar code is
  // loaded only when a file arrives.
  import type { Component } from 'svelte'
  import { onMount, onDestroy } from 'svelte'
  import { _ } from '$lib/i18n'
  import { toasts } from '$lib/stores/toast'
  import { icsPrefill, type EventPrefill, type ICSEvent } from '$lib/calendar/icsPrefill'
  // @ts-ignore - wailsjs path
  import { GetPendingICS } from '../../../../wailsjs/go/app/App'
  // @ts-ignore - wailsjs path
  import { EventsOn, EventsOff } from '../../../../wailsjs/runtime/runtime'

  let ComposerDialog = $state<Component<any> | null>(null)
  let prefill = $state<EventPrefill | null>(null)
  let open = $state(false)

  async function show(ev: ICSEvent) {
    try {
      const [{ default: dialog }, { calendarSources }] = await Promise.all([
        import('$extensions/calendar/frontend/components/EventComposerDialog.svelte'),
        import('$extensions/calendar/frontend/stores/calendarSources.svelte'),
      ])

      if (calendarSources.sources.length === 0) await calendarSources.load()
      const hasWritable = calendarSources.sources.some((src: any) =>
        src.writable && (calendarSources.calendarsBySource[src.id] || []).some((c: any) => c.writable !== false),
      )
      if (!hasWritable) {
        toasts.info($_('addToCalendar.noCalendar'))
        return
      }

      const notes = [$_('icsOpen.fromFile', { values: { file: ev.fileName } })]
      if (ev.recurring) notes.push($_('icsOpen.recurring'))
      if (ev.eventCount > 1) notes.push($_('icsOpen.multiple', { values: { count: ev.eventCount } }))

      prefill = icsPrefill(ev, $_('icsOpen.organizer'), notes.join(' '))
      ComposerDialog = dialog as unknown as Component<any>
      open = true
    } catch (err) {
      console.error('Failed to open the event form for an .ics file:', err)
      toasts.error($_('addToCalendar.failed'))
    }
  }

  onMount(async () => {
    EventsOn('ics:open', (ev: ICSEvent) => {
      show(ev)
    })
    EventsOn('ics:error', (data: { fileName: string; error: string }) => {
      toasts.error($_('icsOpen.failed', { values: { file: data.fileName, error: data.error } }))
    })

    try {
      const ev = await GetPendingICS()
      if (ev) await show(ev)
    } catch (err) {
      toasts.error($_('icsOpen.failed', { values: { file: '', error: String(err).replace(/^Error:\s*/, '') } }))
    }
  })

  onDestroy(() => {
    EventsOff('ics:open')
    EventsOff('ics:error')
  })
</script>

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
    onSaved={() => { open = false; toasts.success($_('icsOpen.added')) }}
  />
{/if}
