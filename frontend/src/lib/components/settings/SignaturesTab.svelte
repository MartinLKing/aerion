<script lang="ts">
  import { onMount } from 'svelte'
  import Icon from '@iconify/svelte'
  import { Button } from '$lib/components/ui/button'
  import { Input } from '$lib/components/ui/input'
  import { Label } from '$lib/components/ui/label'
  import * as Select from '$lib/components/ui/select'
  import ConfirmDialog from '$lib/components/ui/confirm-dialog/ConfirmDialog.svelte'
  import SignatureEditor from './account/SignatureEditor.svelte'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { addToast } from '$lib/stores/toast'
  import { _ } from '$lib/i18n'
  // @ts-ignore - wailsjs path
  import { GetNamedSignatures, SaveNamedSignature, DeleteNamedSignature, GetAccountSignatureDefault, SetAccountSignatureDefault } from '../../../../wailsjs/go/app/App'
  // @ts-ignore - wailsjs path
  import { settings } from '../../../../wailsjs/go/models'

  // Value for "no default". The dropdown can't use an empty string for an item.
  const NO_DEFAULT = 'none'

  let signatures = $state<settings.NamedSignature[]>([])
  let accountDefaults = $state<Record<string, string>>({})
  let loading = $state(true)

  // Editor form. editingId is '' for a new signature, null when the form is closed.
  let editingId = $state<string | null>(null)
  let editName = $state('')
  let editHtml = $state('')
  let saving = $state(false)

  let deleting = $state<settings.NamedSignature | null>(null)
  let showDeleteDialog = $state(false)

  onMount(load)

  async function load() {
    loading = true
    try {
      signatures = (await GetNamedSignatures()) || []
      await loadAccountDefaults()
    } catch (err) {
      console.error('Failed to load signatures:', err)
      addToast({ type: 'error', message: $_('settingsSignatures.failedToLoad') })
    } finally {
      loading = false
    }
  }

  async function loadAccountDefaults() {
    const defaults: Record<string, string> = {}
    for (const acc of accountStore.accounts) {
      defaults[acc.account.id] = (await GetAccountSignatureDefault(acc.account.id)) || NO_DEFAULT
    }
    accountDefaults = defaults
  }

  function openEditor(signature?: settings.NamedSignature) {
    editingId = signature?.id ?? ''
    editName = signature?.name ?? ''
    editHtml = signature?.html ?? ''
  }

  function closeEditor() {
    editingId = null
  }

  async function handleSave() {
    if (!editName.trim()) return
    saving = true
    try {
      await SaveNamedSignature(new settings.NamedSignature({ id: editingId, name: editName, html: editHtml }))
      signatures = (await GetNamedSignatures()) || []
      closeEditor()
    } catch (err) {
      console.error('Failed to save signature:', err)
      addToast({ type: 'error', message: $_('settingsSignatures.failedToSave') })
    } finally {
      saving = false
    }
  }

  function confirmDelete(signature: settings.NamedSignature) {
    deleting = signature
    showDeleteDialog = true
  }

  async function handleDelete() {
    if (!deleting) return
    try {
      await DeleteNamedSignature(deleting.id)
      signatures = (await GetNamedSignatures()) || []
      await loadAccountDefaults()
    } catch (err) {
      console.error('Failed to delete signature:', err)
      addToast({ type: 'error', message: $_('settingsSignatures.failedToDelete') })
    } finally {
      deleting = null
      showDeleteDialog = false
    }
  }

  async function handleDefaultChange(accountId: string, value: string) {
    try {
      await SetAccountSignatureDefault(accountId, value === NO_DEFAULT ? '' : value)
      accountDefaults[accountId] = value
    } catch (err) {
      console.error('Failed to set default signature:', err)
      addToast({ type: 'error', message: $_('settingsSignatures.failedToSave') })
    }
  }

  function defaultLabel(accountId: string): string {
    const value = accountDefaults[accountId]
    return signatures.find(s => s.id === value)?.name ?? $_('settingsSignatures.useAddressSignature')
  }
</script>

{#if loading}
  <div class="flex items-center justify-center py-8">
    <Icon icon="mdi:loading" class="w-6 h-6 animate-spin text-muted-foreground" />
  </div>
{:else if editingId !== null}
  <!-- Add / edit form -->
  <div class="space-y-4">
    <div class="space-y-2">
      <Label for="signatureName">{$_('settingsSignatures.name')}</Label>
      <Input id="signatureName" bind:value={editName} placeholder={$_('settingsSignatures.namePlaceholder')} />
    </div>
    <div class="space-y-2">
      <Label>{$_('settingsSignatures.content')}</Label>
      <SignatureEditor value={editHtml} onchange={(html) => editHtml = html} />
    </div>
    <div class="flex items-center justify-end gap-2">
      <Button variant="ghost" onclick={closeEditor} disabled={saving}>{$_('common.cancel')}</Button>
      <Button onclick={handleSave} disabled={saving || !editName.trim()}>
        {#if saving}
          <Icon icon="mdi:loading" class="w-4 h-4 mr-2 animate-spin" />
        {/if}
        {$_('common.save')}
      </Button>
    </div>
  </div>
{:else}
  <div class="space-y-6">
    <!-- Signature list -->
    <div class="space-y-4">
      <div class="flex items-center justify-between">
        <div>
          <h3 class="text-sm font-medium flex items-center gap-2">
            <Icon icon="mdi:signature-text" class="w-4 h-4" />
            {$_('settingsSignatures.title')}
          </h3>
          <p class="text-xs text-muted-foreground mt-1">{$_('settingsSignatures.help')}</p>
        </div>
        <Button size="sm" onclick={() => openEditor()}>
          <Icon icon="mdi:plus" class="w-4 h-4 mr-1" />
          {$_('settingsSignatures.add')}
        </Button>
      </div>

      {#if signatures.length === 0}
        <p class="text-sm text-muted-foreground text-center py-4">{$_('settingsSignatures.empty')}</p>
      {:else}
        <div class="space-y-2">
          {#each signatures as signature (signature.id)}
            <div class="flex items-center gap-3 p-3 rounded-lg border border-border bg-card">
              <span class="flex-1 min-w-0 font-medium text-sm truncate">{signature.name}</span>
              <Button variant="ghost" size="sm" class="h-8 w-8 p-0" title={$_('common.edit')} onclick={() => openEditor(signature)}>
                <Icon icon="mdi:pencil" class="w-4 h-4" />
              </Button>
              <Button variant="ghost" size="sm" class="h-8 w-8 p-0 text-destructive hover:text-destructive" title={$_('common.delete')} onclick={() => confirmDelete(signature)}>
                <Icon icon="mdi:delete" class="w-4 h-4" />
              </Button>
            </div>
          {/each}
        </div>
      {/if}
    </div>

    <!-- Default per account -->
    {#if signatures.length > 0 && accountStore.accounts.length > 0}
      <div class="space-y-3 pt-4 border-t border-border">
        <div>
          <h3 class="text-sm font-medium">{$_('settingsSignatures.defaultsTitle')}</h3>
          <p class="text-xs text-muted-foreground mt-1">{$_('settingsSignatures.defaultsHelp')}</p>
        </div>
        {#each accountStore.accounts as acc (acc.account.id)}
          <div class="flex items-center gap-3">
            <span class="flex-1 min-w-0 text-sm truncate" title={acc.account.email}>{acc.account.name || acc.account.email}</span>
            <div class="w-56 flex-shrink-0">
              <Select.Root
                value={accountDefaults[acc.account.id] ?? NO_DEFAULT}
                onValueChange={(value) => handleDefaultChange(acc.account.id, value)}
              >
                <Select.Trigger class="h-8">
                  <Select.Value placeholder={$_('settingsSignatures.useAddressSignature')}>
                    <span class="truncate">{defaultLabel(acc.account.id)}</span>
                  </Select.Value>
                </Select.Trigger>
                <Select.Content>
                  <Select.Item value={NO_DEFAULT} label={$_('settingsSignatures.useAddressSignature')} />
                  {#each signatures as signature (signature.id)}
                    <Select.Item value={signature.id} label={signature.name} />
                  {/each}
                </Select.Content>
              </Select.Root>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>
{/if}

<ConfirmDialog
  bind:open={showDeleteDialog}
  title={$_('settingsSignatures.deleteTitle')}
  description={$_('settingsSignatures.deleteDescription', { values: { name: deleting?.name ?? '' } })}
  confirmLabel={$_('common.delete')}
  cancelLabel={$_('common.cancel')}
  onConfirm={handleDelete}
  onCancel={() => { deleting = null; showDeleteDialog = false }}
/>
