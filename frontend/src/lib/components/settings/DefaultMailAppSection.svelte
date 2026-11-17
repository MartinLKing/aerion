<script lang="ts">
  // Settings → General → Default email app (Emailio addition).
  // Registers the running Emailio as a mail app for the current Windows user,
  // so it can be chosen for mailto links in Windows Settings → Default apps.
  import Icon from '@iconify/svelte'
  import { onMount } from 'svelte'
  import { Button } from '$lib/components/ui/button'
  import { _ } from '$lib/i18n'
  import { toasts } from '$lib/stores/toast'
  import {
    GetDefaultMailAppStatus,
    RegisterAsMailApp,
    UnregisterAsMailApp,
    OpenDefaultAppsSettings,
    // @ts-ignore - wailsjs path
  } from '../../../../wailsjs/go/app/App'

  type Status = {
    supported: boolean
    registered: boolean
    registeredPath: string
    currentPath: string
    upToDate: boolean
  }

  let status = $state<Status | null>(null)
  let busy = $state(false)

  async function refresh() {
    try {
      status = await GetDefaultMailAppStatus()
    } catch (err) {
      console.error('Failed to read default mail app status:', err)
    }
  }

  onMount(refresh)

  async function run(action: () => Promise<void>, successKey: string) {
    if (busy) return
    busy = true
    try {
      await action()
      toasts.success($_(successKey))
    } catch (err) {
      toasts.error(String(err).replace(/^Error:\s*/, ''))
    } finally {
      busy = false
      await refresh()
    }
  }
</script>

{#if status?.supported}
  <div class="border-t border-border"></div>

  <div class="space-y-4">
    <h3 class="text-sm font-medium flex items-center gap-2">
      <Icon icon="mdi:email-check-outline" class="w-4 h-4" />
      {$_('settingsGeneral.defaultMailApp.title')}
    </h3>

    <div class="space-y-2">
      <p class="text-xs text-muted-foreground">
        {$_('settingsGeneral.defaultMailApp.help')}
      </p>

      <div class="flex items-start gap-2 text-sm">
        {#if status.upToDate}
          <Icon icon="mdi:check-circle" class="w-4 h-4 text-green-600 shrink-0 mt-0.5" />
          <span>{$_('settingsGeneral.defaultMailApp.registered')}</span>
        {:else if status.registered}
          <Icon icon="mdi:alert" class="w-4 h-4 text-yellow-500 shrink-0 mt-0.5" />
          <span class="min-w-0">
            {$_('settingsGeneral.defaultMailApp.otherCopy')}
            <span class="block text-xs text-muted-foreground break-all">{status.registeredPath}</span>
          </span>
        {:else}
          <Icon icon="mdi:information-outline" class="w-4 h-4 text-muted-foreground shrink-0 mt-0.5" />
          <span>{$_('settingsGeneral.defaultMailApp.notRegistered')}</span>
        {/if}
      </div>

      <div class="flex flex-wrap gap-2 pt-1">
        {#if !status.upToDate}
          <Button size="sm" disabled={busy} onclick={() => run(RegisterAsMailApp, 'settingsGeneral.defaultMailApp.registeredToast')}>
            <Icon icon="mdi:email-check-outline" class="w-4 h-4 mr-1" />
            {$_(status.registered ? 'settingsGeneral.defaultMailApp.useThisCopy' : 'settingsGeneral.defaultMailApp.register')}
          </Button>
        {:else}
          <Button size="sm" variant="outline" disabled={busy} onclick={() => run(OpenDefaultAppsSettings, 'settingsGeneral.defaultMailApp.openedToast')}>
            <Icon icon="mdi:open-in-new" class="w-4 h-4 mr-1" />
            {$_('settingsGeneral.defaultMailApp.openSettings')}
          </Button>
        {/if}
        {#if status.registered}
          <Button size="sm" variant="ghost" disabled={busy} onclick={() => run(UnregisterAsMailApp, 'settingsGeneral.defaultMailApp.removedToast')}>
            {$_('settingsGeneral.defaultMailApp.remove')}
          </Button>
        {/if}
      </div>
    </div>
  </div>
{/if}
