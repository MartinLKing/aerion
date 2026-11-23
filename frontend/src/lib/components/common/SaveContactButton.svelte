<script lang="ts">
  // Small icon next to an email address: saves it as a contact, or renames the
  // contact (Emailio addition). Hidden for the user's own addresses.
  import { onMount, untrack } from 'svelte'
  import Icon from '@iconify/svelte'
  import * as Dialog from '$lib/components/ui/dialog'
  import { Button } from '$lib/components/ui/button'
  import { Input } from '$lib/components/ui/input'
  import { Label } from '$lib/components/ui/label'
  import { dialogGuardOpen, dialogGuardClose } from '$lib/stores/dialogGuard'
  import { contactSaved, contactSavesVersion, listenForContactChanges } from '$lib/stores/contactSaves.svelte'
  import type { ComposerApi } from '$lib/composerApi'
  import { toasts } from '$lib/stores/toast'
  import { _ } from '$lib/i18n'

  interface Props {
    email: string
    /** Name carried by the email, offered when the contact has none */
    name?: string
    api: Pick<ComposerApi, 'getContactSavedState' | 'saveContact'>
    onSaved?: (name: string) => void
  }

  let { email, name = '', api, onSaved }: Props = $props()

  let saved = $state<{ state: string; name: string } | null>(null)
  let open = $state(false)
  let nameInput = $state('')
  let busy = $state(false)
  let error = $state('')

  onMount(listenForContactChanges)

  $effect(() => {
    const address = email
    contactSavesVersion()
    let current = true
    untrack(() => api.getContactSavedState(address))
      .then((s) => { if (current) saved = s })
      .catch((err) => console.error('Failed to look up contact:', err))
    return () => { current = false }
  })

  $effect(() => {
    if (!open) return
    dialogGuardOpen()
    return () => dialogGuardClose()
  })

  const isSaved = $derived(saved?.state === 'saved')
  const isSynced = $derived(saved?.state === 'synced')
  const label = $derived(
    isSynced ? $_('saveContact.synced') : isSaved ? $_('saveContact.rename') : $_('saveContact.save'),
  )

  function handleClick(e: MouseEvent) {
    e.stopPropagation()
    if (isSynced) {
      toasts.info($_('saveContact.synced'))
      return
    }
    nameInput = saved?.name || (name.toLowerCase() === email.toLowerCase() ? '' : name)
    error = ''
    open = true
  }

  async function submit(e?: Event) {
    e?.preventDefault()
    const chosen = nameInput.trim()
    if (!chosen || busy) return
    busy = true
    error = ''
    try {
      await api.saveContact(email, chosen)
      open = false
      contactSaved()
      onSaved?.(chosen)
      toasts.success($_('saveContact.done'))
    } catch (err) {
      error = String(err).replace(/^Error:\s*/, '')
    } finally {
      busy = false
    }
  }
</script>

{#if saved && saved.state !== 'own'}
  <button
    type="button"
    class="inline-flex flex-shrink-0 {isSaved || isSynced ? 'text-primary' : 'text-muted-foreground'} hover:text-foreground"
    title={label}
    aria-label={label}
    onclick={handleClick}
    onkeydown={(e) => e.stopPropagation()}
  >
    <Icon icon={isSaved || isSynced ? 'mdi:account-check' : 'mdi:account-plus-outline'} class="w-4 h-4" />
  </button>

  <Dialog.Root bind:open>
    <Dialog.Content class="max-w-sm">
      <Dialog.Header>
        <Dialog.Title>{isSaved ? $_('saveContact.rename') : $_('saveContact.save')}</Dialog.Title>
        <Dialog.Description class="break-all">{email}</Dialog.Description>
      </Dialog.Header>
      <form class="space-y-2" onsubmit={submit}>
        <Label for="save-contact-name">{$_('saveContact.name')}</Label>
        <Input id="save-contact-name" bind:value={nameInput} autofocus disabled={busy} />
        {#if error}
          <p class="text-sm text-destructive">{error}</p>
        {/if}
      </form>
      <Dialog.Footer>
        <Button variant="ghost" onclick={() => open = false} disabled={busy}>{$_('common.cancel')}</Button>
        <Button onclick={() => submit()} disabled={busy || !nameInput.trim()}>{$_('common.save')}</Button>
      </Dialog.Footer>
    </Dialog.Content>
  </Dialog.Root>
{/if}
