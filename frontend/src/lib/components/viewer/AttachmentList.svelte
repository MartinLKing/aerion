<script lang="ts">
  import Icon from '@iconify/svelte'
  import { GetAttachments, SaveAttachmentAs, SaveAllAttachments, OpenAttachment, SaveEncryptedAttachmentAs, OpenEncryptedAttachment, SaveAllEncryptedAttachments, OpenFile, OpenFolder } from '../../../../wailsjs/go/app/App'
  // @ts-ignore - wailsjs path
  import { message as messageModels } from '../../../../wailsjs/go/models'
  import { toasts } from '$lib/stores/toast'
  import { _ } from '$lib/i18n'

  // Decrypted attachment metadata (from encrypted messages, not in DB)
  interface DecryptedAttachment {
    filename: string
    contentType: string
    size: number
    isInline: boolean
    contentId: string
  }

  interface Props {
    messageId: string
    encryptedAttachments?: DecryptedAttachment[]
  }

  let { messageId, encryptedAttachments }: Props = $props()

  // State
  let attachments = $state<messageModels.Attachment[]>([])
  let downloadingIds = $state<Set<string>>(new Set())
  let savingAll = $state(false)

  // Determine if we're in encrypted mode
  let isEncrypted = $derived(encryptedAttachments != null && encryptedAttachments.length > 0)

  // Chips only show real attachments: inline images (signature logos, embedded
  // pictures) are already visible in the message body.
  let visibleAttachments = $derived(attachments.filter(att => !att.isInline))
  let visibleEncrypted = $derived((encryptedAttachments ?? []).filter(att => !att.isInline))
  let visibleCount = $derived(isEncrypted ? visibleEncrypted.length : visibleAttachments.length)
  let saveAllCount = $derived(isEncrypted ? encryptedAttachments!.length : attachments.length)

  // Load attachments when messageId changes (only for non-encrypted)
  $effect(() => {
    if (encryptedAttachments && encryptedAttachments.length > 0) {
      attachments = []
      return
    }
    if (messageId) {
      loadAttachments(messageId)
    } else {
      attachments = []
    }
  })

  async function loadAttachments(msgId: string) {
    try {
      const result = await GetAttachments(msgId)
      attachments = result || []
    } catch (err) {
      console.error('Failed to load attachments:', err)
      attachments = []
    }
  }

  async function handleDownload(att: messageModels.Attachment) {
    downloadingIds = new Set([...downloadingIds, att.id])
    try {
      const path = await SaveAttachmentAs(att.id)
      if (path) {
        toasts.success($_('toast.attachmentSaved', { values: { filename: att.filename } }), [
          {
            label: $_('attachment.openFile'),
            onClick: () => OpenFile(path)
          },
          {
            label: $_('attachment.showInFolder'),
            onClick: () => OpenFolder(path)
          }
        ])
      }
    } catch (err) {
      console.error('Failed to save attachment:', err)
      toasts.error($_('toast.failedToSaveAttachment', { values: { filename: att.filename } }))
    } finally {
      downloadingIds = new Set([...downloadingIds].filter(id => id !== att.id))
    }
  }

  async function handleOpen(att: messageModels.Attachment) {
    downloadingIds = new Set([...downloadingIds, att.id])
    try {
      await OpenAttachment(att.id)
    } catch (err) {
      console.error('Failed to open attachment:', err)
      toasts.error($_('toast.failedToOpenAttachment', { values: { filename: att.filename } }))
    } finally {
      downloadingIds = new Set([...downloadingIds].filter(id => id !== att.id))
    }
  }

  async function handleEncryptedDownload(att: DecryptedAttachment) {
    const key = att.filename
    downloadingIds = new Set([...downloadingIds, key])
    try {
      const path = await SaveEncryptedAttachmentAs(messageId, att.filename)
      if (path) {
        toasts.success($_('toast.attachmentSaved', { values: { filename: att.filename } }), [
          {
            label: $_('attachment.openFile'),
            onClick: () => OpenFile(path)
          },
          {
            label: $_('attachment.showInFolder'),
            onClick: () => OpenFolder(path)
          }
        ])
      }
    } catch (err) {
      console.error('Failed to save encrypted attachment:', err)
      toasts.error($_('toast.failedToSaveAttachment', { values: { filename: att.filename } }))
    } finally {
      downloadingIds = new Set([...downloadingIds].filter(id => id !== key))
    }
  }

  async function handleEncryptedOpen(att: DecryptedAttachment) {
    const key = att.filename
    downloadingIds = new Set([...downloadingIds, key])
    try {
      await OpenEncryptedAttachment(messageId, att.filename)
    } catch (err) {
      console.error('Failed to open encrypted attachment:', err)
      toasts.error($_('toast.failedToOpenAttachment', { values: { filename: att.filename } }))
    } finally {
      downloadingIds = new Set([...downloadingIds].filter(id => id !== key))
    }
  }

  async function handleSaveAll() {
    savingAll = true
    try {
      if (isEncrypted) {
        const folder = await SaveAllEncryptedAttachments(messageId)
        if (folder) {
          toasts.success($_('toast.attachmentsSaved', { values: { count: encryptedAttachments!.length } }), [
            {
              label: $_('attachment.openFolder'),
              onClick: () => OpenFolder(folder)
            }
          ])
        }
      } else {
        const folder = await SaveAllAttachments(messageId)
        if (folder) {
          toasts.success($_('toast.attachmentsSaved', { values: { count: attachments.length } }), [
            {
              label: $_('attachment.openFolder'),
              onClick: () => OpenFolder(folder)
            }
          ])
        }
      }
    } catch (err) {
      console.error('Failed to save all attachments:', err)
      toasts.error($_('toast.failedToSaveAttachments'))
    } finally {
      savingAll = false
    }
  }

  function getFileIcon(contentType: string): string {
    if (contentType.startsWith('image/')) return 'mdi:file-image'
    if (contentType.startsWith('video/')) return 'mdi:file-video'
    if (contentType.startsWith('audio/')) return 'mdi:file-music'
    if (contentType === 'application/pdf') return 'mdi:file-pdf-box'
    if (contentType.includes('word') || contentType === 'application/msword') return 'mdi:file-word'
    if (contentType.includes('excel') || contentType === 'application/vnd.ms-excel') return 'mdi:file-excel'
    if (contentType.includes('powerpoint') || contentType === 'application/vnd.ms-powerpoint') return 'mdi:file-powerpoint'
    if (contentType.includes('zip') || contentType.includes('compressed')) return 'mdi:folder-zip'
    if (contentType === 'text/plain') return 'mdi:file-document-outline'
    if (contentType === 'text/html') return 'mdi:language-html5'
    return 'mdi:file'
  }

  function formatSize(bytes: number): string {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
  }
</script>

{#snippet chip(filename: string, contentType: string, size: number, isBusy: boolean, onOpen: () => void, onSave: () => void)}
  <div class="inline-flex items-center gap-1.5 max-w-full rounded-md border border-border bg-muted/30 pl-2 pr-1 py-0.5 text-xs">
    <Icon icon={getFileIcon(contentType)} class="w-4 h-4 text-primary flex-shrink-0" />
    <span class="font-medium text-foreground truncate" title={filename}>{filename}</span>
    <span class="text-muted-foreground whitespace-nowrap flex-shrink-0">{formatSize(size)}</span>
    {#if isBusy}
      <div class="p-1 flex-shrink-0">
        <Icon icon="mdi:loading" class="w-4 h-4 animate-spin text-muted-foreground" />
      </div>
    {:else}
      <button class="p-1 rounded hover:bg-muted transition-colors flex-shrink-0" title={$_('attachment.open')} onclick={onOpen}>
        <Icon icon="mdi:open-in-new" class="w-4 h-4 text-muted-foreground" />
      </button>
      <button class="p-1 rounded hover:bg-muted transition-colors flex-shrink-0" title={$_('attachment.download')} onclick={onSave}>
        <Icon icon="mdi:download" class="w-4 h-4 text-muted-foreground" />
      </button>
    {/if}
  </div>
{/snippet}

{#if visibleCount > 0}
  <!-- The chips sit inside the clickable message header: stop clicks and
       Enter/Space here from also expanding or collapsing the message. -->
  <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
  <div
    class="mt-2 flex flex-wrap items-center gap-2 cursor-default"
    role="group"
    aria-label={$_('viewer.attachments')}
    onclick={(e) => e.stopPropagation()}
    onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') e.stopPropagation() }}
  >
    {#if isEncrypted}
      {#each visibleEncrypted as att (att.filename)}
        {@render chip(att.filename, att.contentType, att.size, downloadingIds.has(att.filename), () => handleEncryptedOpen(att), () => handleEncryptedDownload(att))}
      {/each}
    {:else}
      {#each visibleAttachments as att (att.id)}
        {@render chip(att.filename, att.contentType, att.size, downloadingIds.has(att.id), () => handleOpen(att), () => handleDownload(att))}
      {/each}
    {/if}

    {#if visibleCount > 1}
      <button
        class="inline-flex items-center gap-1.5 px-2 py-1 text-xs text-muted-foreground hover:text-foreground hover:bg-muted rounded-md transition-colors border border-dashed border-border"
        onclick={handleSaveAll}
        disabled={savingAll}
      >
        {#if savingAll}
          <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />
          {$_('attachment.saving')}
        {:else}
          <Icon icon="mdi:download-multiple" class="w-4 h-4" />
          {$_('attachment.saveAll', { values: { count: saveAllCount } })}
        {/if}
      </button>
    {/if}
  </div>
{/if}
