package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Named signatures are global (not tied to an account or identity) and stored
// as JSON in the settings table rather than in their own table. This fork
// avoids adding database migrations so they can't clash with upstream's
// sequentially numbered ones.
const (
	namedSignaturesKey            = "named_signatures"
	accountSignatureDefaultPrefix = "signature_default_"
)

// NamedSignature is a reusable signature that can be picked in the composer.
type NamedSignature struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	HTML string `json:"html"`
}

// GetNamedSignatures returns all named signatures in the order they were created.
func (s *Store) GetNamedSignatures() ([]NamedSignature, error) {
	raw, err := s.Get(namedSignaturesKey)
	if err != nil {
		return nil, err
	}
	signatures := []NamedSignature{}
	if raw == "" {
		return signatures, nil
	}
	if err := json.Unmarshal([]byte(raw), &signatures); err != nil {
		return nil, fmt.Errorf("failed to parse named signatures: %w", err)
	}
	return signatures, nil
}

// SaveNamedSignature adds a signature, or updates the one with a matching ID.
// A new ID is assigned when the signature has none.
func (s *Store) SaveNamedSignature(signature NamedSignature) (NamedSignature, error) {
	signature.Name = strings.TrimSpace(signature.Name)
	if signature.Name == "" {
		return NamedSignature{}, errors.New("signature name is required")
	}

	signatures, err := s.GetNamedSignatures()
	if err != nil {
		return NamedSignature{}, err
	}

	if signature.ID == "" {
		signature.ID = uuid.New().String()
		signatures = append(signatures, signature)
	} else {
		found := false
		for i := range signatures {
			if signatures[i].ID == signature.ID {
				signatures[i] = signature
				found = true
				break
			}
		}
		if !found {
			return NamedSignature{}, fmt.Errorf("signature not found: %s", signature.ID)
		}
	}

	if err := s.writeNamedSignatures(signatures); err != nil {
		return NamedSignature{}, err
	}
	return signature, nil
}

// DeleteNamedSignature removes a signature. Any account default pointing at it
// is ignored from then on (see GetAccountSignatureDefault).
func (s *Store) DeleteNamedSignature(id string) error {
	signatures, err := s.GetNamedSignatures()
	if err != nil {
		return err
	}
	remaining := make([]NamedSignature, 0, len(signatures))
	for _, signature := range signatures {
		if signature.ID != id {
			remaining = append(remaining, signature)
		}
	}
	return s.writeNamedSignatures(remaining)
}

// GetAccountSignatureDefault returns the ID of the account's default named
// signature, or "" if none is set or that signature has been deleted.
func (s *Store) GetAccountSignatureDefault(accountID string) (string, error) {
	id, err := s.Get(accountSignatureDefaultPrefix + accountID)
	if err != nil || id == "" {
		return "", err
	}
	signatures, err := s.GetNamedSignatures()
	if err != nil {
		return "", err
	}
	for _, signature := range signatures {
		if signature.ID == id {
			return id, nil
		}
	}
	return "", nil
}

// SetAccountSignatureDefault sets the account's default named signature. An
// empty ID clears it.
func (s *Store) SetAccountSignatureDefault(accountID, signatureID string) error {
	return s.Set(accountSignatureDefaultPrefix+accountID, signatureID)
}

func (s *Store) writeNamedSignatures(signatures []NamedSignature) error {
	data, err := json.Marshal(signatures)
	if err != nil {
		return fmt.Errorf("failed to encode named signatures: %w", err)
	}
	return s.Set(namedSignaturesKey, string(data))
}
