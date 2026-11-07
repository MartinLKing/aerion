<script lang="ts">
  import { onMount } from 'svelte'
  import Icon from '@iconify/svelte'
  import * as Select from '$lib/components/ui/select'
  import { supportedLocales, _ } from '$lib/i18n'
  import type { ComposerApi } from '$lib/composerApi'
  import { getSpellcheckEnabled } from '$lib/stores/settings.svelte'
  import { DICT_NAMES, SPELLCHECK_DICTS } from './locales'
  import { getActiveSpellLanguage, setActiveSpellLanguage } from './settings'

  interface Props {
    api: ComposerApi
  }

  let { api }: Props = $props()

  // Stored and shown when checking against every language switched on in Settings
  const ALL = 'all'
  // Used until the user has picked a language
  const DEFAULT_LANGUAGE = 'en'
  // The user's everyday languages first, then the other bundled dictionaries
  const PREFERRED = ['fr', 'en', 'en-gb', 'de', 'es']

  const languages = [
    ...PREFERRED,
    ...SPELLCHECK_DICTS.filter((code) => !PREFERRED.includes(code)),
  ].map((code) => ({
    code,
    name: DICT_NAMES[code] ?? supportedLocales.find((l) => l.code === code)?.name ?? code,
  }))

  let value = $state(getActiveSpellLanguage() || ALL)

  onMount(async () => {
    try {
      const saved = await api.getSpellcheckActiveLanguage()
      const code = saved === ALL ? '' : saved || DEFAULT_LANGUAGE
      // Ignore a saved code whose dictionary is no longer bundled
      const known = code === '' || languages.some((l) => l.code === code)
      apply(known ? code : DEFAULT_LANGUAGE)
    } catch (err) {
      console.error('Failed to load spell-check language:', err)
    }
  })

  function apply(code: string) {
    value = code || ALL
    if (code !== getActiveSpellLanguage()) setActiveSpellLanguage(code)
  }

  function handleChange(selected: string) {
    apply(selected === ALL ? '' : selected)
    api.setSpellcheckActiveLanguage(selected).catch((err) => {
      console.error('Failed to save spell-check language:', err)
    })
  }

  const label = $derived(
    value === ALL ? $_('composer.spellLanguageAll') : (languages.find((l) => l.code === value)?.name ?? value)
  )
</script>

{#if getSpellcheckEnabled()}
  <div class="w-36 flex-shrink-0" title={$_('composer.spellLanguage')}>
    <Select.Root {value} onValueChange={handleChange}>
      <Select.Trigger class="h-8 px-2 text-sm border-0 bg-transparent shadow-none focus:ring-0">
        <Select.Value placeholder={$_('composer.spellLanguage')}>
          <span class="flex items-center gap-1.5 min-w-0">
            <Icon icon="mdi:spellcheck" class="w-4 h-4 flex-shrink-0 text-muted-foreground" />
            <span class="truncate">{label}</span>
          </span>
        </Select.Value>
      </Select.Trigger>
      <Select.Content>
        {#each languages as lang (lang.code)}
          <Select.Item value={lang.code} label={lang.name} />
        {/each}
        <Select.Item value={ALL} label={$_('composer.spellLanguageAll')} />
      </Select.Content>
    </Select.Root>
  </div>
{/if}
