<script lang="ts">
  // Asks for a folder name, for creating or renaming a folder.
  import * as Dialog from '$lib/components/ui/dialog'
  import { Button } from '$lib/components/ui/button'
  import { Input } from '$lib/components/ui/input'
  import { Label } from '$lib/components/ui/label'
  import Icon from '@iconify/svelte'
  import { dialogGuardOpen, dialogGuardClose } from '$lib/stores/dialogGuard'
  import { _ } from '$lib/i18n'

  interface Props {
    open: boolean
    title: string
    /** Where the folder goes, such as "Inside Work", shown under the title */
    description?: string
    initialName?: string
    confirmLabel: string
    /** Saves the name; a thrown error is shown in the dialog */
    onSubmit: (name: string) => Promise<void>
  }

  let { open = $bindable(false), title, description = '', initialName = '', confirmLabel, onSubmit }: Props = $props()

  let name = $state('')
  let busy = $state(false)
  let error = $state('')

  $effect(() => {
    if (!open) return
    name = initialName
    error = ''
    busy = false
    dialogGuardOpen()
    return () => dialogGuardClose()
  })

  async function submit(e?: Event) {
    e?.preventDefault()
    if (!name.trim() || busy) return
    busy = true
    error = ''
    try {
      await onSubmit(name.trim())
      open = false
    } catch (err) {
      error = String(err).replace(/^Error:\s*/, '')
    } finally {
      busy = false
    }
  }
</script>

<Dialog.Root bind:open>
  <Dialog.Content class="max-w-sm">
    <Dialog.Header>
      <Dialog.Title>{title}</Dialog.Title>
      {#if description}
        <Dialog.Description>{description}</Dialog.Description>
      {/if}
    </Dialog.Header>
    <form class="space-y-2" onsubmit={submit}>
      <Label for="folder-name">{$_('folders.name')}</Label>
      <Input id="folder-name" bind:value={name} autofocus disabled={busy} />
      {#if error}
        <p class="text-sm text-destructive">{error}</p>
      {/if}
    </form>
    <Dialog.Footer>
      <Button variant="ghost" onclick={() => open = false} disabled={busy}>{$_('common.cancel')}</Button>
      <Button onclick={() => submit()} disabled={busy || !name.trim()}>
        {#if busy}
          <Icon icon="mdi:loading" class="w-4 h-4 mr-2 animate-spin" />
        {/if}
        {confirmLabel}
      </Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>
