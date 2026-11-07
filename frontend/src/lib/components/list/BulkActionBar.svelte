<script lang="ts">
  import { onMount } from 'svelte'
  import Icon from '@iconify/svelte'
  // @ts-ignore - wailsjs path
  import { Archive, GetAccounts, MarkAsRead, MarkAsUnread, MoveToFolder, Star, Undo, Unstar } from '../../../../wailsjs/go/app/App'
  // @ts-ignore - wailsjs path
  import { account } from '../../../../wailsjs/go/models'
  import FolderPickerDialog from '$lib/components/common/FolderPickerDialog.svelte'
  import { dialogGuardClose, dialogGuardOpen } from '$lib/stores/dialogGuard'
  import { toasts } from '$lib/stores/toast'
  import { _ } from '$lib/i18n'

  interface Props {
    /** Every message in the ticked conversations */
    messageIds: string[]
    /** How many conversations are ticked */
    count: number
    /** How many conversations are in the list, to know when all are ticked */
    totalCount: number
    hasUnstarred: boolean
    hasUnread: boolean
    /** True when Delete would permanently delete rather than move to Trash */
    deleteIsPermanent: boolean
    /** Account the folder picker starts on */
    initialAccountId: string
    /** Folder the picker leaves out, because the messages are already in it */
    currentFolderId: string
    onDelete: () => void
    onSelectAll: () => void
    onClear: () => void
    /** Refresh the list. removed is true when messages left it, so ticks should clear. */
    onActionComplete: (removed: boolean) => void
  }

  let {
    messageIds,
    count,
    totalCount,
    hasUnstarred,
    hasUnread,
    deleteIsPermanent,
    initialAccountId,
    currentFolderId,
    onDelete,
    onSelectAll,
    onClear,
    onActionComplete,
  }: Props = $props()

  let accounts = $state<account.Account[]>([])
  let showFolderPicker = $state(false)

  onMount(async () => {
    try {
      accounts = (await GetAccounts()) || []
    } catch (err) {
      console.error('Failed to load accounts:', err)
    }
  })

  // Stop background reloads from dismissing the picker
  $effect(() => {
    if (showFolderPicker) {
      dialogGuardOpen()
      return () => dialogGuardClose()
    }
  })

  // A button keeping focus would swallow the list's keyboard shortcuts
  function releaseFocus() {
    ;(document.activeElement as HTMLElement)?.blur?.()
  }

  async function handleUndo() {
    try {
      const description = await Undo()
      toasts.success($_('toast.undone', { values: { description } }))
    } catch (err) {
      console.error('Undo failed:', err)
      toasts.error($_('toast.undoFailed'))
    }
  }

  async function handleArchive() {
    releaseFocus()
    try {
      await Archive(messageIds)
      toasts.success($_('toast.archived'), [{ label: $_('common.undo'), onClick: handleUndo }])
      onActionComplete(true)
    } catch (err) {
      console.error('Archive failed:', err)
      toasts.error($_('toast.failedToArchive'))
    }
  }

  function handleDelete() {
    releaseFocus()
    onDelete()
  }

  async function handleToggleRead() {
    releaseFocus()
    try {
      if (hasUnread) {
        await MarkAsRead(messageIds)
        toasts.success($_('toast.markedAsRead'))
      } else {
        await MarkAsUnread(messageIds)
        toasts.success($_('toast.markedAsUnread'))
      }
      onActionComplete(false)
    } catch (err) {
      console.error('Read status toggle failed:', err)
      toasts.error($_('toast.failedToUpdateReadStatus'))
    }
  }

  async function handleToggleStar() {
    releaseFocus()
    try {
      if (hasUnstarred) {
        await Star(messageIds)
        toasts.success($_('toast.starred'))
      } else {
        await Unstar(messageIds)
        toasts.success($_('toast.starRemoved'))
      }
      onActionComplete(false)
    } catch (err) {
      console.error('Star toggle failed:', err)
      toasts.error($_('toast.failedToUpdateStar'))
    }
  }

  async function handleFolderSelected(folderId: string, folderName: string) {
    showFolderPicker = false
    try {
      await MoveToFolder(messageIds, folderId)
      toasts.success($_('toast.movedTo', { values: { folder: folderName } }), [{ label: $_('common.undo'), onClick: handleUndo }])
      onActionComplete(true)
    } catch (err) {
      console.error('Move failed:', err)
      toasts.error($_('toast.failedToMove'))
    }
  }

  const buttonClass = 'inline-flex items-center gap-1 px-2 py-1 text-xs rounded-md hover:bg-muted transition-colors whitespace-nowrap'
</script>

<div class="flex flex-wrap items-center gap-x-1 gap-y-1 px-4 py-2 bg-primary/10 border-b border-border" role="toolbar" aria-label={$_('messageList.selectionActions')}>
  <span class="text-xs font-medium text-foreground mr-1 whitespace-nowrap">
    {$_('messageList.selectedCount', { values: { count } })}
  </span>

  <button class={buttonClass} onclick={handleArchive}>
    <Icon icon="mdi:archive-outline" class="w-4 h-4" />
    {$_('contextMenu.archive')}
  </button>
  <button class={buttonClass} onclick={handleDelete}>
    <Icon icon={deleteIsPermanent ? 'mdi:delete-forever' : 'mdi:delete-outline'} class="w-4 h-4" />
    {$_(deleteIsPermanent ? 'contextMenu.deletePermanently' : 'contextMenu.delete')}
  </button>
  <button class={buttonClass} onclick={handleToggleRead}>
    <Icon icon={hasUnread ? 'mdi:email-open-outline' : 'mdi:email-outline'} class="w-4 h-4" />
    {$_(hasUnread ? 'contextMenu.markAsRead' : 'contextMenu.markAsUnread')}
  </button>
  <button class={buttonClass} onclick={handleToggleStar}>
    <Icon icon={hasUnstarred ? 'mdi:star-outline' : 'mdi:star'} class="w-4 h-4 {hasUnstarred ? '' : 'text-yellow-500'}" />
    {$_(hasUnstarred ? 'contextMenu.star' : 'contextMenu.removeStar')}
  </button>
  <button class={buttonClass} onclick={() => { releaseFocus(); showFolderPicker = true }}>
    <Icon icon="mdi:folder-move-outline" class="w-4 h-4" />
    {$_('contextMenu.moveTo')}
  </button>

  <div class="flex items-center gap-1 ml-auto">
    {#if count < totalCount}
      <button class="{buttonClass} text-primary" onclick={() => { releaseFocus(); onSelectAll() }}>
        {$_('messageList.selectAllLoaded')}
      </button>
    {/if}
    <button class="{buttonClass} text-muted-foreground" onclick={() => { releaseFocus(); onClear() }}>
      <Icon icon="mdi:close" class="w-4 h-4" />
      {$_('messageList.clearSelection')}
    </button>
  </div>
</div>

<FolderPickerDialog
  bind:open={showFolderPicker}
  title={$_('contextMenu.moveTo')}
  {initialAccountId}
  {accounts}
  excludeFolderId={currentFolderId}
  onSelect={handleFolderSelected}
/>
