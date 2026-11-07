<script lang="ts">
  import { onMount } from 'svelte'
  import Icon from '@iconify/svelte'
  import { Button } from '$lib/components/ui/button'
  import { Input } from '$lib/components/ui/input'
  import { Label } from '$lib/components/ui/label'
  import * as Select from '$lib/components/ui/select'
  import { addToast } from '$lib/stores/toast'
  import { _ } from '$lib/i18n'
  import { translationLanguageName } from '$lib/translation/languages'
  // @ts-ignore - wailsjs path
  import { GetTranslationSettings, SaveTranslationSettings, RemoveTranslationKey, TestTranslation } from '../../../../wailsjs/go/app/App'

  // Saved on its own, separately from the dialog's Save button
  let configured = $state(false)
  let key = $state('')
  let region = $state('global')
  let language = $state('en')
  let languages = $state<string[]>(['en', 'fr', 'de', 'es'])
  let busy = $state(false)
  let testResult = $state('')
  let testError = $state('')

  onMount(load)

  async function load() {
    try {
      const s = await GetTranslationSettings()
      configured = s.configured
      region = s.region
      language = s.language
      languages = s.languages?.length ? s.languages : languages
    } catch (err) {
      console.error('Failed to load translation settings:', err)
    }
  }

  async function handleSave() {
    busy = true
    testResult = ''
    testError = ''
    try {
      await SaveTranslationSettings(key, region, language)
      key = ''
      await load()
      addToast({ type: 'success', message: $_('translation.saved') })
    } catch (err) {
      addToast({ type: 'error', message: `${$_('translation.failedToSave')}: ${err}` })
    } finally {
      busy = false
    }
  }

  async function handleTest() {
    busy = true
    testResult = ''
    testError = ''
    try {
      testResult = await TestTranslation()
    } catch (err) {
      testError = String(err)
    } finally {
      busy = false
    }
  }

  async function handleRemove() {
    busy = true
    try {
      await RemoveTranslationKey()
      testResult = ''
      testError = ''
      await load()
    } catch (err) {
      addToast({ type: 'error', message: `${$_('translation.failedToSave')}: ${err}` })
    } finally {
      busy = false
    }
  }
</script>

<div class="space-y-4">
  <div>
    <h3 class="text-sm font-medium flex items-center gap-2">
      <Icon icon="mdi:translate" class="w-4 h-4" />
      {$_('translation.title')}
    </h3>
    <p class="text-xs text-muted-foreground mt-1">{$_('translation.help')}</p>
  </div>

  <div class="space-y-2">
    <Label for="translation-key">{$_('translation.key')}</Label>
    <Input
      id="translation-key"
      type="password"
      autocomplete="off"
      bind:value={key}
      placeholder={configured ? $_('translation.keySavedPlaceholder') : $_('translation.keyPlaceholder')}
    />
  </div>

  <div class="flex gap-3">
    <div class="flex-1 space-y-2">
      <Label for="translation-region">{$_('translation.region')}</Label>
      <Input id="translation-region" bind:value={region} placeholder="global" />
      <p class="text-xs text-muted-foreground">{$_('translation.regionHelp')}</p>
    </div>
    <div class="w-44 space-y-2">
      <Label>{$_('translation.defaultLanguage')}</Label>
      <Select.Root value={language} onValueChange={(v) => language = v}>
        <Select.Trigger>
          <Select.Value placeholder={translationLanguageName(language)}>{translationLanguageName(language)}</Select.Value>
        </Select.Trigger>
        <Select.Content>
          {#each languages as code (code)}
            <Select.Item value={code} label={translationLanguageName(code)} />
          {/each}
        </Select.Content>
      </Select.Root>
    </div>
  </div>

  <div class="flex items-center gap-2 flex-wrap">
    <Button size="sm" onclick={handleSave} disabled={busy || (!configured && !key.trim())}>
      {$_('translation.save')}
    </Button>
    <Button size="sm" variant="outline" onclick={handleTest} disabled={busy || !configured}>
      {#if busy}
        <Icon icon="mdi:loading" class="w-4 h-4 mr-1 animate-spin" />
      {/if}
      {$_('translation.test')}
    </Button>
    {#if configured}
      <Button size="sm" variant="ghost" class="text-destructive hover:text-destructive" onclick={handleRemove} disabled={busy}>
        {$_('translation.removeKey')}
      </Button>
      <span class="text-xs text-muted-foreground flex items-center gap-1">
        <Icon icon="mdi:check-circle-outline" class="w-4 h-4 text-green-600" />
        {$_('translation.keySaved')}
      </span>
    {/if}
  </div>

  {#if testResult}
    <p class="text-sm text-green-700 dark:text-green-400">{$_('translation.testWorked', { values: { result: testResult } })}</p>
  {/if}
  {#if testError}
    <p class="text-sm text-destructive">{testError}</p>
  {/if}
</div>
