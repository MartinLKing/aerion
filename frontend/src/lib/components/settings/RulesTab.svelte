<script lang="ts">
  import { onMount } from 'svelte'
  import Icon from '@iconify/svelte'
  import { Button } from '$lib/components/ui/button'
  import { Input } from '$lib/components/ui/input'
  import { Label } from '$lib/components/ui/label'
  import * as Select from '$lib/components/ui/select'
  import Switch from '$lib/components/ui/switch/Switch.svelte'
  import ConfirmDialog from '$lib/components/ui/confirm-dialog/ConfirmDialog.svelte'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { addToast } from '$lib/stores/toast'
  import { _ } from '$lib/i18n'
  // @ts-ignore - wailsjs path
  import { GetFilterRules, SaveFilterRule, DeleteFilterRule, MoveFilterRule, CountFilterRuleMatches, RunFilterRule, GetFolders } from '../../../../wailsjs/go/app/App'
  // @ts-ignore - wailsjs path
  import { rules, folder } from '../../../../wailsjs/go/models'

  // The dropdowns can't use an empty string for an item
  const ANY = 'any'
  const ALL_ACCOUNTS = 'all'
  const ACTION_NONE = 'none'
  const ACTION_ARCHIVE = 'archive'
  const ACTION_MOVE = 'move'
  // Folders it makes no sense to file mail into
  const HIDDEN_FOLDER_TYPES = ['inbox', 'drafts', 'all', 'starred']

  let ruleList = $state<rules.Rule[]>([])
  let foldersByAccount = $state<Record<string, folder.Folder[]>>({})
  let loading = $state(true)

  // Add / edit form. editingId is '' for a new rule, null when the form is closed.
  let editingId = $state<string | null>(null)
  let editEnabled = $state(true)
  let editAccount = $state(ALL_ACCOUNTS)
  let editSenderMode = $state(ANY)
  let editSenderValue = $state('')
  let editSubjectMode = $state(ANY)
  let editSubjectValue = $state('')
  let editAction = $state(ACTION_ARCHIVE)
  let editFolderId = $state('')
  let editMarkRead = $state(false)
  let saving = $state(false)
  let formError = $state('')

  let deleting = $state<rules.Rule | null>(null)
  let showDeleteDialog = $state(false)

  let running = $state<rules.Rule | null>(null)
  let runCount = $state(0)
  let showRunDialog = $state(false)
  let busyRuleId = $state<string | null>(null)

  const editFolders = $derived(
    (foldersByAccount[editAccount] ?? []).filter(f => !HIDDEN_FOLDER_TYPES.includes(f.type))
  )
  const canSave = $derived(
    (editSenderMode !== ANY && !!editSenderValue.trim() || editSubjectMode !== ANY && !!editSubjectValue.trim()) &&
    (editSenderMode === ANY || !!editSenderValue.trim()) &&
    (editSubjectMode === ANY || !!editSubjectValue.trim()) &&
    (editAction !== ACTION_NONE || editMarkRead) &&
    (editAction !== ACTION_MOVE || (editAccount !== ALL_ACCOUNTS && !!editFolderId))
  )

  onMount(load)

  async function load() {
    loading = true
    try {
      ruleList = (await GetFilterRules()) || []
      const folders: Record<string, folder.Folder[]> = {}
      for (const acc of accountStore.accounts) {
        folders[acc.account.id] = (await GetFolders(acc.account.id)) || []
      }
      foldersByAccount = folders
    } catch (err) {
      console.error('Failed to load rules:', err)
      addToast({ type: 'error', message: $_('settingsRules.failedToLoad') })
    } finally {
      loading = false
    }
  }

  async function reloadRules() {
    ruleList = (await GetFilterRules()) || []
  }

  function openEditor(rule?: rules.Rule) {
    formError = ''
    editingId = rule?.id ?? ''
    editEnabled = rule?.enabled ?? true
    editAccount = rule?.accountId || ALL_ACCOUNTS
    editSenderMode = rule?.senderMode || ANY
    editSenderValue = rule?.senderValue ?? ''
    editSubjectMode = rule?.subjectMode || ANY
    editSubjectValue = rule?.subjectValue ?? ''
    editFolderId = rule?.moveToFolderId ?? ''
    editMarkRead = rule?.markRead ?? false
    editAction = !rule || rule.archive ? ACTION_ARCHIVE : rule.moveToFolderId ? ACTION_MOVE : ACTION_NONE
  }

  function closeEditor() {
    editingId = null
  }

  function handleAccountChange(value: string) {
    editAccount = value
    // Folders belong to one account, so an earlier choice no longer applies
    editFolderId = ''
    if (value === ALL_ACCOUNTS && editAction === ACTION_MOVE) {
      editAction = ACTION_ARCHIVE
    }
  }

  async function handleSave() {
    if (!canSave) return
    saving = true
    formError = ''
    try {
      await SaveFilterRule(new rules.Rule({
        id: editingId,
        enabled: editEnabled,
        accountId: editAccount === ALL_ACCOUNTS ? '' : editAccount,
        senderMode: editSenderMode === ANY ? '' : editSenderMode,
        senderValue: editSenderMode === ANY ? '' : editSenderValue,
        subjectMode: editSubjectMode === ANY ? '' : editSubjectMode,
        subjectValue: editSubjectMode === ANY ? '' : editSubjectValue,
        archive: editAction === ACTION_ARCHIVE,
        moveToFolderId: editAction === ACTION_MOVE ? editFolderId : '',
        markRead: editMarkRead,
      }))
      await reloadRules()
      closeEditor()
    } catch (err) {
      console.error('Failed to save rule:', err)
      // The backend's validation messages explain what to fix
      formError = String(err)
    } finally {
      saving = false
    }
  }

  async function handleToggle(rule: rules.Rule, enabled: boolean) {
    try {
      await SaveFilterRule(new rules.Rule({ ...rule, enabled }))
      await reloadRules()
    } catch (err) {
      console.error('Failed to update rule:', err)
      addToast({ type: 'error', message: $_('settingsRules.failedToSave') })
    }
  }

  async function handleMove(rule: rules.Rule, delta: number) {
    try {
      await MoveFilterRule(rule.id, delta)
      await reloadRules()
    } catch (err) {
      console.error('Failed to reorder rule:', err)
      addToast({ type: 'error', message: $_('settingsRules.failedToSave') })
    }
  }

  function confirmDelete(rule: rules.Rule) {
    deleting = rule
    showDeleteDialog = true
  }

  async function handleDelete() {
    if (!deleting) return
    try {
      await DeleteFilterRule(deleting.id)
      await reloadRules()
    } catch (err) {
      console.error('Failed to delete rule:', err)
      addToast({ type: 'error', message: $_('settingsRules.failedToDelete') })
    } finally {
      deleting = null
      showDeleteDialog = false
    }
  }

  // Count first, so nothing moves until the user has seen how much will
  async function prepareRun(rule: rules.Rule) {
    busyRuleId = rule.id
    try {
      runCount = await CountFilterRuleMatches(rule.id)
      if (runCount === 0) {
        addToast({ type: 'info', message: $_('settingsRules.noMatches') })
        return
      }
      running = rule
      showRunDialog = true
    } catch (err) {
      console.error('Failed to count rule matches:', err)
      addToast({ type: 'error', message: $_('settingsRules.failedToRun') })
    } finally {
      busyRuleId = null
    }
  }

  async function handleRun() {
    if (!running) return
    const rule = running
    running = null
    showRunDialog = false
    busyRuleId = rule.id
    try {
      const count = await RunFilterRule(rule.id)
      addToast({ type: 'success', message: $_('settingsRules.ranOn', { values: { count } }) })
    } catch (err) {
      console.error('Failed to run rule:', err)
      addToast({ type: 'error', message: `${$_('settingsRules.failedToRun')}: ${err}` })
    } finally {
      busyRuleId = null
    }
  }

  function accountLabel(accountId: string): string {
    const acc = accountStore.accounts.find(a => a.account.id === accountId)?.account
    return acc?.name || acc?.email || $_('settingsRules.unknownAccount')
  }

  function folderLabel(accountId: string, folderId: string): string {
    return foldersByAccount[accountId]?.find(f => f.id === folderId)?.name ?? $_('settingsRules.unknownFolder')
  }

  function modeLabel(mode: string): string {
    switch (mode) {
      case 'contains': return $_('settingsRules.modeContains')
      case 'equals': return $_('settingsRules.modeEquals')
      case 'domain': return $_('settingsRules.modeDomain')
      default: return $_('settingsRules.modeAny')
    }
  }

  function actionLabel(action: string): string {
    switch (action) {
      case ACTION_ARCHIVE: return $_('settingsRules.actionArchive')
      case ACTION_MOVE: return $_('settingsRules.actionMove')
      default: return $_('settingsRules.actionNone')
    }
  }

  function conditionSummary(rule: rules.Rule): string {
    const parts: string[] = []
    if (rule.senderMode) {
      parts.push(`${$_('settingsRules.sender')} ${modeLabel(rule.senderMode).toLowerCase()} "${rule.senderValue}"`)
    }
    if (rule.subjectMode) {
      parts.push(`${$_('settingsRules.subject')} ${modeLabel(rule.subjectMode).toLowerCase()} "${rule.subjectValue}"`)
    }
    return parts.join(` ${$_('settingsRules.and')} `)
  }

  function actionSummary(rule: rules.Rule): string {
    const parts: string[] = []
    if (rule.archive) parts.push($_('settingsRules.actionArchive'))
    if (rule.moveToFolderId) {
      parts.push($_('settingsRules.moveToNamed', { values: { folder: folderLabel(rule.accountId, rule.moveToFolderId) } }))
    }
    if (rule.markRead) parts.push($_('settingsRules.markRead'))
    return parts.join(', ')
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
      <Label>{$_('settingsRules.appliesTo')}</Label>
      <Select.Root value={editAccount} onValueChange={handleAccountChange}>
        <Select.Trigger>
          <Select.Value placeholder={$_('settingsRules.allAccounts')}>
            {editAccount === ALL_ACCOUNTS ? $_('settingsRules.allAccounts') : accountLabel(editAccount)}
          </Select.Value>
        </Select.Trigger>
        <Select.Content>
          <Select.Item value={ALL_ACCOUNTS} label={$_('settingsRules.allAccounts')} />
          {#each accountStore.accounts as acc (acc.account.id)}
            <Select.Item value={acc.account.id} label={acc.account.name || acc.account.email} />
          {/each}
        </Select.Content>
      </Select.Root>
    </div>

    <div class="space-y-2">
      <Label for="ruleSender">{$_('settingsRules.senderHeading')}</Label>
      <div class="flex items-center gap-2">
        <div class="w-44 flex-shrink-0">
          <Select.Root value={editSenderMode} onValueChange={(v) => editSenderMode = v}>
            <Select.Trigger>
              <Select.Value placeholder={$_('settingsRules.modeAny')}>{modeLabel(editSenderMode)}</Select.Value>
            </Select.Trigger>
            <Select.Content>
              <Select.Item value={ANY} label={$_('settingsRules.modeAny')} />
              <Select.Item value="contains" label={$_('settingsRules.modeContains')} />
              <Select.Item value="equals" label={$_('settingsRules.modeEquals')} />
              <Select.Item value="domain" label={$_('settingsRules.modeDomain')} />
            </Select.Content>
          </Select.Root>
        </div>
        <Input
          id="ruleSender"
          bind:value={editSenderValue}
          disabled={editSenderMode === ANY}
          placeholder={editSenderMode === 'domain' ? 'amazon.de' : $_('settingsRules.senderPlaceholder')}
        />
      </div>
    </div>

    <div class="space-y-2">
      <Label for="ruleSubject">{$_('settingsRules.subjectHeading')}</Label>
      <div class="flex items-center gap-2">
        <div class="w-44 flex-shrink-0">
          <Select.Root value={editSubjectMode} onValueChange={(v) => editSubjectMode = v}>
            <Select.Trigger>
              <Select.Value placeholder={$_('settingsRules.modeAny')}>{modeLabel(editSubjectMode)}</Select.Value>
            </Select.Trigger>
            <Select.Content>
              <Select.Item value={ANY} label={$_('settingsRules.modeAny')} />
              <Select.Item value="contains" label={$_('settingsRules.modeContains')} />
              <Select.Item value="equals" label={$_('settingsRules.modeEquals')} />
            </Select.Content>
          </Select.Root>
        </div>
        <Input id="ruleSubject" bind:value={editSubjectValue} disabled={editSubjectMode === ANY} />
      </div>
    </div>

    <div class="space-y-2">
      <Label>{$_('settingsRules.thenHeading')}</Label>
      <div class="flex items-center gap-2">
        <div class="w-44 flex-shrink-0">
          <Select.Root value={editAction} onValueChange={(v) => editAction = v}>
            <Select.Trigger>
              <Select.Value placeholder={actionLabel(ACTION_NONE)}>{actionLabel(editAction)}</Select.Value>
            </Select.Trigger>
            <Select.Content>
              <Select.Item value={ACTION_ARCHIVE} label={actionLabel(ACTION_ARCHIVE)} />
              <Select.Item value={ACTION_MOVE} label={actionLabel(ACTION_MOVE)} disabled={editAccount === ALL_ACCOUNTS} />
              <Select.Item value={ACTION_NONE} label={actionLabel(ACTION_NONE)} />
            </Select.Content>
          </Select.Root>
        </div>
        {#if editAction === ACTION_MOVE}
          <div class="flex-1 min-w-0">
            <Select.Root value={editFolderId} onValueChange={(v) => editFolderId = v}>
              <Select.Trigger>
                <Select.Value placeholder={$_('settingsRules.chooseFolder')}>
                  {editFolderId ? folderLabel(editAccount, editFolderId) : $_('settingsRules.chooseFolder')}
                </Select.Value>
              </Select.Trigger>
              <Select.Content>
                {#each editFolders as f (f.id)}
                  <Select.Item value={f.id} label={f.name} />
                {/each}
              </Select.Content>
            </Select.Root>
          </div>
        {/if}
      </div>
      <label class="flex items-center gap-2 cursor-pointer text-sm pt-1">
        <input type="checkbox" bind:checked={editMarkRead} class="w-4 h-4 rounded border-input accent-primary" />
        {$_('settingsRules.markRead')}
      </label>
      {#if editAccount === ALL_ACCOUNTS}
        <p class="text-xs text-muted-foreground">{$_('settingsRules.moveNeedsAccount')}</p>
      {/if}
    </div>

    {#if formError}
      <p class="text-sm text-destructive">{formError}</p>
    {/if}

    <div class="flex items-center justify-end gap-2">
      <Button variant="ghost" onclick={closeEditor} disabled={saving}>{$_('common.cancel')}</Button>
      <Button onclick={handleSave} disabled={saving || !canSave}>
        {#if saving}
          <Icon icon="mdi:loading" class="w-4 h-4 mr-2 animate-spin" />
        {/if}
        {$_('common.save')}
      </Button>
    </div>
  </div>
{:else}
  <div class="space-y-4">
    <div class="flex items-center justify-between gap-4">
      <div>
        <h3 class="text-sm font-medium flex items-center gap-2">
          <Icon icon="mdi:filter-outline" class="w-4 h-4" />
          {$_('settingsRules.title')}
        </h3>
        <p class="text-xs text-muted-foreground mt-1">{$_('settingsRules.help')}</p>
      </div>
      <Button size="sm" class="flex-shrink-0" onclick={() => openEditor()}>
        <Icon icon="mdi:plus" class="w-4 h-4 mr-1" />
        {$_('settingsRules.add')}
      </Button>
    </div>

    {#if ruleList.length === 0}
      <p class="text-sm text-muted-foreground text-center py-4">{$_('settingsRules.empty')}</p>
    {:else}
      <div class="space-y-2">
        {#each ruleList as rule, index (rule.id)}
          <div class="flex items-center gap-3 p-3 rounded-lg border border-border bg-card {rule.enabled ? '' : 'opacity-60'}">
            <Switch checked={rule.enabled} onCheckedChange={(v: boolean) => handleToggle(rule, v)} />
            <div class="flex-1 min-w-0">
              <div class="text-sm font-medium truncate" title={conditionSummary(rule)}>{conditionSummary(rule)}</div>
              <div class="text-xs text-muted-foreground truncate">
                {actionSummary(rule)} · {rule.accountId ? accountLabel(rule.accountId) : $_('settingsRules.allAccounts')}
              </div>
            </div>
            <div class="flex items-center gap-1 flex-shrink-0">
              <Button variant="ghost" size="sm" class="h-8 w-8 p-0" title={$_('settingsRules.moveUp')} disabled={index === 0} onclick={() => handleMove(rule, -1)}>
                <Icon icon="mdi:arrow-up" class="w-4 h-4" />
              </Button>
              <Button variant="ghost" size="sm" class="h-8 w-8 p-0" title={$_('settingsRules.moveDown')} disabled={index === ruleList.length - 1} onclick={() => handleMove(rule, 1)}>
                <Icon icon="mdi:arrow-down" class="w-4 h-4" />
              </Button>
              <Button variant="ghost" size="sm" class="h-8 w-8 p-0" title={$_('settingsRules.runOnExisting')} disabled={busyRuleId !== null} onclick={() => prepareRun(rule)}>
                {#if busyRuleId === rule.id}
                  <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />
                {:else}
                  <Icon icon="mdi:play" class="w-4 h-4" />
                {/if}
              </Button>
              <Button variant="ghost" size="sm" class="h-8 w-8 p-0" title={$_('common.edit')} onclick={() => openEditor(rule)}>
                <Icon icon="mdi:pencil" class="w-4 h-4" />
              </Button>
              <Button variant="ghost" size="sm" class="h-8 w-8 p-0 text-destructive hover:text-destructive" title={$_('common.delete')} onclick={() => confirmDelete(rule)}>
                <Icon icon="mdi:delete" class="w-4 h-4" />
              </Button>
            </div>
          </div>
        {/each}
      </div>
      <p class="text-xs text-muted-foreground">{$_('settingsRules.orderHelp')}</p>
    {/if}
  </div>
{/if}

<ConfirmDialog
  bind:open={showDeleteDialog}
  title={$_('settingsRules.deleteTitle')}
  description={deleting ? conditionSummary(deleting) : ''}
  confirmLabel={$_('common.delete')}
  cancelLabel={$_('common.cancel')}
  onConfirm={handleDelete}
  onCancel={() => { deleting = null; showDeleteDialog = false }}
/>

<ConfirmDialog
  bind:open={showRunDialog}
  title={$_('settingsRules.runTitle')}
  description={running ? $_('settingsRules.runDescription', { values: { count: runCount, action: actionSummary(running) } }) : ''}
  confirmLabel={$_('settingsRules.runConfirm')}
  cancelLabel={$_('common.cancel')}
  onConfirm={handleRun}
  onCancel={() => { running = null; showRunDialog = false }}
/>
