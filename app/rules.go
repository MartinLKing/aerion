package app

import (
	"fmt"
	stdsync "sync"
	"time"

	"github.com/hkdb/aerion/internal/folder"
	"github.com/hkdb/aerion/internal/logging"
	"github.com/hkdb/aerion/internal/message"
	"github.com/hkdb/aerion/internal/rules"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// filterRulesMu makes sure two syncs finishing together can't both act on the
// same new messages.
var filterRulesMu stdsync.Mutex

// ============================================================================
// Filter rule API - Exposed to frontend via Wails bindings
// ============================================================================

// GetFilterRules returns all filter rules in the order they're applied.
func (a *App) GetFilterRules() ([]rules.Rule, error) {
	return a.settingsStore.GetFilterRules()
}

// SaveFilterRule creates or updates a filter rule and returns it.
func (a *App) SaveFilterRule(rule rules.Rule) (rules.Rule, error) {
	return a.settingsStore.SaveFilterRule(rule)
}

// DeleteFilterRule removes a filter rule.
func (a *App) DeleteFilterRule(id string) error {
	return a.settingsStore.DeleteFilterRule(id)
}

// MoveFilterRule moves a rule one place earlier (delta -1) or later (delta 1).
func (a *App) MoveFilterRule(id string, delta int) error {
	return a.settingsStore.MoveFilterRule(id, delta)
}

// CountFilterRuleMatches returns how many downloaded Inbox messages a rule would
// act on, so the user can confirm before running it.
func (a *App) CountFilterRuleMatches(ruleID string) (int, error) {
	rule, err := a.filterRuleByID(ruleID)
	if err != nil {
		return 0, err
	}
	matches, err := a.existingRuleMatches(rule)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, m := range matches {
		count += len(m)
	}
	return count, nil
}

// RunFilterRule applies a rule to the downloaded messages in Inbox and returns
// how many it acted on.
func (a *App) RunFilterRule(ruleID string) (int, error) {
	filterRulesMu.Lock()
	defer filterRulesMu.Unlock()

	rule, err := a.filterRuleByID(ruleID)
	if err != nil {
		return 0, err
	}
	matches, err := a.existingRuleMatches(rule)
	if err != nil {
		return 0, err
	}

	acted := 0
	for accountID, candidates := range matches {
		if err := a.applyRuleActions(rule, accountID, candidates); err != nil {
			return acted, err
		}
		acted += len(candidates)
	}
	return acted, nil
}

func (a *App) filterRuleByID(ruleID string) (rules.Rule, error) {
	list, err := a.settingsStore.GetFilterRules()
	if err != nil {
		return rules.Rule{}, err
	}
	for _, rule := range list {
		if rule.ID == ruleID {
			return rule, nil
		}
	}
	return rules.Rule{}, fmt.Errorf("rule not found: %s", ruleID)
}

// existingRuleMatches returns the downloaded Inbox messages a rule matches, by account.
func (a *App) existingRuleMatches(rule rules.Rule) (map[string][]rules.Candidate, error) {
	accounts, err := a.accountStore.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list accounts: %w", err)
	}

	matches := make(map[string][]rules.Candidate)
	for _, acc := range accounts {
		if !acc.Enabled || !rule.AppliesToAccount(acc.ID) {
			continue
		}
		inbox, err := a.folderStore.GetByType(acc.ID, folder.TypeInbox)
		if err != nil || inbox == nil {
			continue
		}
		candidates, err := a.messageStore.ListRuleCandidates(inbox.ID, 0)
		if err != nil {
			return nil, err
		}
		if matched := rules.MatchAll(rule, candidates); len(matched) > 0 {
			matches[acc.ID] = matched
		}
	}
	return matches, nil
}

// ============================================================================
// Applying rules to new mail
// ============================================================================

// applyFilterRulesAfterSync runs rules if the folder that just synced is an Inbox.
func (a *App) applyFilterRulesAfterSync(accountID, folderID string) {
	f, err := a.folderStore.Get(folderID)
	if err != nil || f == nil || f.Type != folder.TypeInbox {
		return
	}
	a.applyFilterRulesToNewMail(accountID)
}

// applyFilterRulesToNewMail runs rules on Inbox messages that have arrived since
// the last check and returns how many it acted on. A rule problem is only logged,
// and never gets in the way of receiving mail.
func (a *App) applyFilterRulesToNewMail(accountID string) int {
	filterRulesMu.Lock()
	defer filterRulesMu.Unlock()

	log := logging.WithComponent("app.rules")

	inbox, err := a.folderStore.GetByType(accountID, folder.TypeInbox)
	if err != nil || inbox == nil {
		return 0
	}
	maxUID, err := a.messageStore.MaxUID(inbox.ID)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read Inbox UIDs")
		return 0
	}

	// The first check on an Inbox only records where things stand, so adding the
	// first rule or a new account doesn't file existing mail. The same goes when
	// the server has renumbered the Inbox (UIDVALIDITY changed).
	validity, markerUID, ok, err := a.settingsStore.GetFilterRuleMarker(inbox.ID)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read rule marker")
		return 0
	}
	if !ok || validity != inbox.UIDValidity {
		if maxUID > 0 {
			if err := a.settingsStore.SetFilterRuleMarker(inbox.ID, inbox.UIDValidity, maxUID); err != nil {
				log.Warn().Err(err).Msg("Failed to save rule marker")
			}
		}
		return 0
	}
	if maxUID <= markerUID {
		return 0
	}

	candidates, err := a.messageStore.ListRuleCandidates(inbox.ID, markerUID)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to list new messages for rules")
		return 0
	}

	// Saved before acting, so a failing rule isn't retried on every sync
	if err := a.settingsStore.SetFilterRuleMarker(inbox.ID, inbox.UIDValidity, maxUID); err != nil {
		log.Warn().Err(err).Msg("Failed to save rule marker")
		return 0
	}

	list, err := a.settingsStore.GetFilterRules()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load filter rules")
		return 0
	}
	alreadyFiled, err := a.settingsStore.GetFiledMessageIDs()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load filed message IDs")
		return 0
	}

	acted := 0
	for _, group := range rules.Select(list, accountID, candidates, alreadyFiled) {
		if err := a.applyRuleActions(group.Rule, accountID, group.Messages); err != nil {
			log.Error().Err(err).Str("rule", group.Rule.ID).Msg("Failed to apply filter rule")
			continue
		}
		acted += len(group.Messages)
	}
	if acted > 0 {
		log.Info().Int("count", acted).Str("account", accountID).Msg("Filter rules acted on new mail")
	}
	return acted
}

// applyRuleActions marks messages read and/or moves them as the rule says.
func (a *App) applyRuleActions(rule rules.Rule, accountID string, candidates []rules.Candidate) error {
	destFolderID, err := a.ruleDestination(rule, accountID)
	if err != nil {
		return err
	}

	ids := make([]string, 0, len(candidates))
	unreadIDs := make([]string, 0, len(candidates))
	messageIDs := make([]string, 0, len(candidates))
	for _, c := range candidates {
		ids = append(ids, c.ID)
		messageIDs = append(messageIDs, c.MessageID)
		if !c.IsRead {
			unreadIDs = append(unreadIDs, c.ID)
		}
	}

	if destFolderID == "" {
		if rule.MarkRead {
			if err := a.setReadStatus(unreadIDs, true); err != nil {
				return err
			}
		}
	} else {
		if rule.MarkRead {
			a.markReadBeforeMove(unreadIDs)
		}
		// Not undoable: Undo should reverse the user's own last action
		if err := a.moveToFolder(ids, destFolderID, false); err != nil {
			return err
		}
	}

	return a.settingsStore.AddFiledMessageIDs(messageIDs)
}

// ruleDestination returns the folder a rule moves mail to for this account, or ""
// if the rule doesn't move mail.
func (a *App) ruleDestination(rule rules.Rule, accountID string) (string, error) {
	if rule.Archive {
		archive, err := a.GetSpecialFolder(accountID, folder.TypeArchive)
		if err != nil {
			return "", fmt.Errorf("failed to get archive folder: %w", err)
		}
		if archive == nil {
			return "", fmt.Errorf("no archive folder for this account")
		}
		return archive.ID, nil
	}
	if rule.MoveToFolderID == "" {
		return "", nil
	}

	dest, err := a.folderStore.Get(rule.MoveToFolderID)
	if err != nil || dest == nil {
		return "", fmt.Errorf("the rule's folder no longer exists")
	}
	if dest.AccountID != accountID {
		return "", fmt.Errorf("the rule's folder belongs to another account")
	}
	return dest.ID, nil
}

// markReadBeforeMove sets messages read locally and on the server, and waits for
// the server. setReadStatus doesn't wait, so a move right after it could take the
// message away first and it would come back unread in the destination folder.
// Failing to set the flag doesn't stop the move.
func (a *App) markReadBeforeMove(messageIDs []string) {
	log := logging.WithComponent("app.rules")

	if len(messageIDs) == 0 {
		return
	}
	messageIDs = a.expandToFolderCopies(messageIDs)

	messages, err := a.messageStore.GetByIDs(messageIDs)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load messages to mark read")
		return
	}
	if len(messages) == 0 {
		return
	}

	byFolder := make(map[string][]*message.Message)
	for _, m := range messages {
		byFolder[m.FolderID] = append(byFolder[m.FolderID], m)
	}

	isRead := true
	if err := a.messageStore.UpdateFlagsBatch(messageIDs, &isRead, nil); err != nil {
		log.Warn().Err(err).Msg("Failed to mark messages read locally")
		return
	}
	wailsRuntime.EventsEmit(a.ctx, "messages:readChanged", map[string]interface{}{
		"messageIds": messageIDs,
		"isRead":     true,
	})

	folderCounts := make(map[string]int)
	for folderID, msgs := range byFolder {
		a.adjustFolderCounts(folderCounts, folderID, 0, readFlipDelta(msgs, true))
	}
	a.emitFolderCounts(folderCounts)

	for folderID, msgs := range byFolder {
		var syncErr error
		for attempt := 1; attempt <= 3; attempt++ {
			if syncErr = a.syncFlagsToIMAP(msgs, folderID, "read", true); syncErr == nil {
				break
			}
			time.Sleep(time.Duration(attempt) * time.Second)
		}
		if syncErr != nil {
			log.Error().Err(syncErr).Str("folderID", folderID).Msg("Failed to mark messages read on the server before moving")
		}
	}
}
