<script lang="ts">
  import Icon from '@iconify/svelte'
  import { _, locale } from '$lib/i18n'
  import { toasts } from '$lib/stores/toast'
  import { detectedLanguageName } from '$lib/translation/languages'

  interface Props {
    x: number
    y: number
    loading: boolean
    text: string
    error: string
    detectedLanguage: string
    onClose: () => void
  }

  let { x, y, loading, text, error, detectedLanguage, onClose }: Props = $props()

  // Keep the panel on screen
  const left = $derived(Math.max(8, Math.min(x, window.innerWidth - 400)))
  const top = $derived(Math.max(8, Math.min(y, window.innerHeight - 260)))

  async function copy() {
    try {
      await navigator.clipboard.writeText(text)
      toasts.success($_('translation.copied'))
    } catch (err) {
      console.error('Failed to copy translation:', err)
    }
  }
</script>

<svelte:window onkeydown={(e) => { if (e.key === 'Escape') onClose() }} />

<!-- Click outside to close -->
<button type="button" class="fixed inset-0 z-40 cursor-default" aria-label={$_('common.close')} onclick={onClose}></button>

<div
  class="fixed z-50 w-96 max-w-[calc(100vw-16px)] bg-popover text-popover-foreground rounded-md shadow-lg border border-border p-3 space-y-2"
  style="left: {left}px; top: {top}px;"
  role="dialog"
  aria-label={$_('translation.translation')}
>
  <div class="flex items-center gap-2 text-xs text-muted-foreground">
    <Icon icon="mdi:translate" class="w-4 h-4" />
    <span class="flex-1">
      {#if detectedLanguage}
        {$_('translation.translatedFrom', { values: { language: detectedLanguageName(detectedLanguage, $locale ?? 'en') } })}
      {:else}
        {$_('translation.translation')}
      {/if}
    </span>
    <button class="p-1 rounded hover:bg-muted" title={$_('common.close')} onclick={onClose}>
      <Icon icon="mdi:close" class="w-4 h-4" />
    </button>
  </div>

  {#if loading}
    <div class="flex items-center gap-2 text-sm text-muted-foreground py-2">
      <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />
      {$_('translation.translating')}
    </div>
  {:else if error}
    <p class="text-sm text-destructive">{error}</p>
  {:else}
    <p class="text-sm whitespace-pre-wrap max-h-48 overflow-y-auto select-text">{text}</p>
    <div class="flex justify-end">
      <button class="inline-flex items-center gap-1 px-2 py-1 text-xs rounded hover:bg-muted" onclick={copy}>
        <Icon icon="mdi:content-copy" class="w-4 h-4" />
        {$_('viewer.copy')}
      </button>
    </div>
  {/if}
</div>
