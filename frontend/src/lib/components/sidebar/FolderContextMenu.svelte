<script lang="ts">
  import Icon from '@iconify/svelte'
  import { ContextMenu as ContextMenuPrimitive } from 'bits-ui'
  import {
    ContextMenuContent,
    ContextMenuItem,
    ContextMenuSeparator,
  } from '$lib/components/ui/context-menu'
  import {
    MarkAllFolderMessagesAsRead,
    MarkAllFolderMessagesAsUnread,
    MarkWholeFolder,
    GetMarkAllCoverage,
  } from '../../../../wailsjs/go/app/App'
  import { toasts } from '$lib/stores/toast'
  import type { Snippet } from 'svelte'
  import { _ } from '$lib/i18n'
  import MarkAllScopeDialog from './MarkAllScopeDialog.svelte'
  import FolderNameDialog from './FolderNameDialog.svelte'
  import { ConfirmDialog } from '$lib/components/ui/confirm-dialog'
  // @ts-ignore - wailsjs path
  import { CreateFolder, RenameFolder, DeleteFolder } from '../../../../wailsjs/go/app/App'
  // @ts-ignore - wailsjs path
  import { folder as folderModels } from '../../../../wailsjs/go/models'
  import { accountStore } from '$lib/stores/accounts.svelte'

  interface Props {
    folderId: string
    /** With these, the user's own folders also get New subfolder, Rename and Delete */
    folder?: folderModels.Folder
    accountId?: string
    children?: Snippet
  }

  let {
    folderId,
    folder,
    accountId = '',
    children,
  }: Props = $props()

  // Only the user's own folders can be changed, never Inbox, Sent, Trash...
  const canManage = $derived(!!folder && !!accountId && folder.type === 'folder')

  let createOpen = $state(false)
  let renameOpen = $state(false)
  let deleteOpen = $state(false)

  async function createSubfolder(name: string) {
    await CreateFolder(accountId, folderId, name)
    await accountStore.loadFolders(accountId)
    toasts.success($_('folders.created', { values: { name } }))
  }

  async function renameFolder(name: string) {
    await RenameFolder(accountId, folderId, name)
    await accountStore.loadFolders(accountId)
    toasts.success($_('folders.renamed', { values: { name } }))
  }

  async function deleteFolder() {
    deleteOpen = false
    try {
      await DeleteFolder(accountId, folderId)
      await accountStore.loadFolders(accountId)
      toasts.success($_('folders.deleted', { values: { name: folder?.name ?? '' } }))
    } catch (err) {
      toasts.error(String(err).replace(/^Error:\s*/, ''))
    }
  }

  // Whole-folder scope prompt (#458): shown only when the server holds mail
  // beyond the sync period that the local-only action would not reach.
  let scopeOpen = $state(false)
  let scopeSeen = $state(true)
  let scopeCount = $state(0)

  // Read-flag changes are not on the undo stack (#243), so no Undo action on
  // these toasts — it used to undo whatever unrelated action was on top.
  async function runLocal(seen: boolean) {
    try {
      if (seen) {
        await MarkAllFolderMessagesAsRead(folderId)
      }
      if (!seen) {
        await MarkAllFolderMessagesAsUnread(folderId)
      }
      toasts.success(seen ? $_('toast.markedAllAsRead') : $_('toast.markedAllAsUnread'))
    } catch (err) {
      console.error('Mark all failed:', err)
      toasts.error(seen ? $_('toast.failedToMarkAllAsRead') : $_('toast.failedToMarkAllAsUnread'))
    }
  }

  async function runWhole(seen: boolean) {
    try {
      await MarkWholeFolder(folderId, seen)
      toasts.success(seen ? $_('toast.markedAllAsRead') : $_('toast.markedAllAsUnread'))
    } catch (err) {
      console.error('Mark whole folder failed:', err)
      toasts.error(seen ? $_('toast.failedToMarkAllAsRead') : $_('toast.failedToMarkAllAsUnread'))
    }
  }

  async function startMarkAll(seen: boolean) {
    let extra = 0
    try {
      extra = await GetMarkAllCoverage(folderId, seen)
    } catch (err) {
      console.error('Mark all coverage check failed:', err)
    }
    if (extra <= 0) {
      await runLocal(seen)
      return
    }
    scopeSeen = seen
    scopeCount = extra
    scopeOpen = true
  }

  function handleMarkAllRead() {
    void startMarkAll(true)
  }

  function handleMarkAllUnread() {
    void startMarkAll(false)
  }
</script>

<MarkAllScopeDialog
  bind:open={scopeOpen}
  seen={scopeSeen}
  count={scopeCount}
  onAll={() => runWhole(scopeSeen)}
  onSynced={() => runLocal(scopeSeen)}
/>

<ContextMenuPrimitive.Root>
  <ContextMenuPrimitive.Trigger>
    {#if children}
      {@render children()}
    {/if}
  </ContextMenuPrimitive.Trigger>

  <ContextMenuContent>
    <ContextMenuItem onSelect={handleMarkAllRead}>
      <Icon icon="mdi:email-check-outline" class="mr-2 h-4 w-4" />
      {$_('contextMenu.markAllAsRead')}
    </ContextMenuItem>
    <ContextMenuItem onSelect={handleMarkAllUnread}>
      <Icon icon="mdi:email-outline" class="mr-2 h-4 w-4" />
      {$_('contextMenu.markAllAsUnread')}
    </ContextMenuItem>
    {#if canManage}
      <ContextMenuSeparator />
      <ContextMenuItem onSelect={() => createOpen = true}>
        <Icon icon="mdi:folder-plus-outline" class="mr-2 h-4 w-4" />
        {$_('folders.newSubfolder')}
      </ContextMenuItem>
      <ContextMenuItem onSelect={() => renameOpen = true}>
        <Icon icon="mdi:pencil-outline" class="mr-2 h-4 w-4" />
        {$_('folders.rename')}
      </ContextMenuItem>
      <ContextMenuItem onSelect={() => deleteOpen = true}>
        <Icon icon="mdi:folder-remove-outline" class="mr-2 h-4 w-4" />
        {$_('folders.delete')}
      </ContextMenuItem>
    {/if}
  </ContextMenuContent>
</ContextMenuPrimitive.Root>

{#if canManage}
  <FolderNameDialog
    bind:open={createOpen}
    title={$_('folders.newSubfolderTitle')}
    description={$_('folders.inside', { values: { name: folder?.name ?? '' } })}
    confirmLabel={$_('folders.create')}
    onSubmit={createSubfolder}
  />
  <FolderNameDialog
    bind:open={renameOpen}
    title={$_('folders.renameTitle')}
    initialName={folder?.name ?? ''}
    confirmLabel={$_('folders.rename')}
    onSubmit={renameFolder}
  />
  <ConfirmDialog
    bind:open={deleteOpen}
    title={$_('folders.deleteTitle', { values: { name: folder?.name ?? '' } })}
    description={$_('folders.deleteDescription')}
    confirmLabel={$_('folders.delete')}
    cancelLabel={$_('common.cancel')}
    variant="destructive"
    onConfirm={deleteFolder}
    onCancel={() => deleteOpen = false}
  />
{/if}
