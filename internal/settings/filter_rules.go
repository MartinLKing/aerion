package settings

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/hkdb/aerion/internal/rules"
)

// Filter rules and their bookkeeping live in the settings table, for the same
// reason as named signatures: no migration to clash with upstream's numbering.
const (
	filterRulesKey         = "filter_rules"
	filterRuleMarkerPrefix = "filter_rule_marker_"
	filterRuleFiledKey     = "filter_rule_filed_ids"

	// maxFiledMessageIDs caps how many filed messages are remembered
	maxFiledMessageIDs = 2000
)

// GetFilterRules returns all rules in the order they're applied.
func (s *Store) GetFilterRules() ([]rules.Rule, error) {
	raw, err := s.Get(filterRulesKey)
	if err != nil {
		return nil, err
	}
	list := []rules.Rule{}
	if raw == "" {
		return list, nil
	}
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return nil, fmt.Errorf("failed to parse filter rules: %w", err)
	}
	return list, nil
}

// SaveFilterRule validates and saves a rule, adding it to the end of the list if
// it has no ID, and returns the saved rule.
func (s *Store) SaveFilterRule(rule rules.Rule) (rules.Rule, error) {
	rule = rule.Normalize()
	if err := rule.Validate(); err != nil {
		return rules.Rule{}, err
	}

	list, err := s.GetFilterRules()
	if err != nil {
		return rules.Rule{}, err
	}

	if rule.ID == "" {
		rule.ID = uuid.New().String()
		list = append(list, rule)
	} else {
		found := false
		for i := range list {
			if list[i].ID == rule.ID {
				list[i] = rule
				found = true
				break
			}
		}
		if !found {
			return rules.Rule{}, fmt.Errorf("rule not found: %s", rule.ID)
		}
	}

	if err := s.writeFilterRules(list); err != nil {
		return rules.Rule{}, err
	}
	return rule, nil
}

// DeleteFilterRule removes a rule.
func (s *Store) DeleteFilterRule(id string) error {
	list, err := s.GetFilterRules()
	if err != nil {
		return err
	}
	remaining := make([]rules.Rule, 0, len(list))
	for _, rule := range list {
		if rule.ID != id {
			remaining = append(remaining, rule)
		}
	}
	return s.writeFilterRules(remaining)
}

// MoveFilterRule moves a rule one place earlier (delta -1) or later (delta 1).
// Moving past either end of the list does nothing.
func (s *Store) MoveFilterRule(id string, delta int) error {
	list, err := s.GetFilterRules()
	if err != nil {
		return err
	}
	for i := range list {
		if list[i].ID != id {
			continue
		}
		target := i + delta
		if target < 0 || target >= len(list) {
			return nil
		}
		list[i], list[target] = list[target], list[i]
		return s.writeFilterRules(list)
	}
	return fmt.Errorf("rule not found: %s", id)
}

func (s *Store) writeFilterRules(list []rules.Rule) error {
	data, err := json.Marshal(list)
	if err != nil {
		return fmt.Errorf("failed to encode filter rules: %w", err)
	}
	return s.Set(filterRulesKey, string(data))
}

// GetFilterRuleMarker returns the highest UID already checked in a folder and the
// folder's UIDVALIDITY at the time. ok is false if nothing has been recorded.
func (s *Store) GetFilterRuleMarker(folderID string) (uidValidity uint32, uid int64, ok bool, err error) {
	raw, err := s.Get(filterRuleMarkerPrefix + folderID)
	if err != nil || raw == "" {
		return 0, 0, false, err
	}
	validityPart, uidPart, found := strings.Cut(raw, ":")
	if !found {
		return 0, 0, false, nil
	}
	validity, validityErr := strconv.ParseUint(validityPart, 10, 32)
	parsedUID, uidErr := strconv.ParseInt(uidPart, 10, 64)
	if validityErr != nil || uidErr != nil {
		return 0, 0, false, nil
	}
	return uint32(validity), parsedUID, true, nil
}

// SetFilterRuleMarker records the highest UID checked in a folder.
func (s *Store) SetFilterRuleMarker(folderID string, uidValidity uint32, uid int64) error {
	return s.Set(filterRuleMarkerPrefix+folderID, fmt.Sprintf("%d:%d", uidValidity, uid))
}

// GetFiledMessageIDs returns the Message-IDs of recently filed messages.
func (s *Store) GetFiledMessageIDs() (map[string]bool, error) {
	list, err := s.filedMessageIDs()
	if err != nil {
		return nil, err
	}
	filed := make(map[string]bool, len(list))
	for _, id := range list {
		filed[id] = true
	}
	return filed, nil
}

// AddFiledMessageIDs remembers filed messages, dropping the oldest past the cap.
func (s *Store) AddFiledMessageIDs(ids []string) error {
	list, err := s.filedMessageIDs()
	if err != nil {
		return err
	}
	known := make(map[string]bool, len(list))
	for _, id := range list {
		known[id] = true
	}
	for _, id := range ids {
		if id != "" && !known[id] {
			known[id] = true
			list = append(list, id)
		}
	}
	if len(list) > maxFiledMessageIDs {
		list = list[len(list)-maxFiledMessageIDs:]
	}
	data, err := json.Marshal(list)
	if err != nil {
		return fmt.Errorf("failed to encode filed message IDs: %w", err)
	}
	return s.Set(filterRuleFiledKey, string(data))
}

func (s *Store) filedMessageIDs() ([]string, error) {
	raw, err := s.Get(filterRuleFiledKey)
	if err != nil || raw == "" {
		return nil, err
	}
	var list []string
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return nil, fmt.Errorf("failed to parse filed message IDs: %w", err)
	}
	return list, nil
}
