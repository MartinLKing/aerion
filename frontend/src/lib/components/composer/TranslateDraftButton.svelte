<script lang="ts">
  import { onMount } from 'svelte'
  import Icon from '@iconify/svelte'
  import * as DropdownMenu from '$lib/components/ui/dropdown-menu'
  import type { ComposerApi } from '$lib/composerApi'
  import { _ } from '$lib/i18n'
  import { translationLanguageName } from '$lib/translation/languages'

  interface Props {
    api: ComposerApi
    /** Translates the user's own text into the given language */
    onTranslate: (to: string) => Promise<void>
  }

  let { api, onTranslate }: Props = $props()

  let configured = $state(false)
  let languages = $state<string[]>([])
  let defaultLanguage = $state('en')
  let busy = $state(false)

  onMount(async () => {
    try {
      const s = await api.getTranslationSettings()
      configured = s.configured
      languages = s.languages ?? []
      defaultLanguage = s.language
    } catch (err) {
      console.error('Failed to load translation settings:', err)
    }
  })

  async function translate(to: string) {
    if (busy) return
    busy = true
    try {
      await onTranslate(to)
    } finally {
      busy = false
    }
  }
</script>

{#if configured}
  <DropdownMenu.Root>
    <DropdownMenu.Trigger
      class="p-1.5 rounded hover:bg-muted transition-colors flex items-center gap-1.5 text-xs"
      title={$_('translation.translateDraft')}
      disabled={busy}
    >
      <Icon icon={busy ? 'mdi:loading' : 'mdi:translate'} class="w-5 h-5 {busy ? 'animate-spin' : ''}" />
      <span class="hidden sm:inline">{$_('translation.translateDraftShort')}</span>
    </DropdownMenu.Trigger>
    <DropdownMenu.Content align="end">
      {#each [defaultLanguage, ...languages.filter((l) => l !== defaultLanguage)] as code (code)}
        <DropdownMenu.Item onSelect={() => translate(code)}>
          {$_('translation.translateTo', { values: { language: translationLanguageName(code) } })}
        </DropdownMenu.Item>
      {/each}
    </DropdownMenu.Content>
  </DropdownMenu.Root>
{/if}
