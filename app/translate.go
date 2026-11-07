package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hkdb/aerion/internal/credentials"
	"github.com/hkdb/aerion/internal/email"
	"github.com/hkdb/aerion/internal/settings"
	"github.com/hkdb/aerion/internal/translate"
)

// The Azure key lives in the credential store (Windows Credential Manager when
// available); the region and default language are ordinary settings.
const (
	translationSecretScope = "translation"
	translationSecretKey   = "azure_key"
	translationRegionKey   = "translation_region"
	translationLanguageKey = "translation_language"

	defaultTranslationRegion   = "global"
	defaultTranslationLanguage = "en"
	translationTimeout         = 60 * time.Second
)

// TranslationSettings is what the UI needs to know. The key itself is never sent back.
type TranslationSettings struct {
	Configured bool     `json:"configured"`
	Region     string   `json:"region"`
	Language   string   `json:"language"`
	Languages  []string `json:"languages"`
}

// TranslatedMessage is a translated email body. AsText is true when the HTML was
// too long to translate and the plain text version was translated instead.
type TranslatedMessage struct {
	HTML             string `json:"html"`
	Text             string `json:"text"`
	AsText           bool   `json:"asText"`
	DetectedLanguage string `json:"detectedLanguage"`
}

func loadTranslationSettings(settingsStore *settings.Store, credStore *credentials.Store) (TranslationSettings, string, error) {
	key, err := credStore.GetExtensionSecret(translationSecretScope, translationSecretKey)
	if err != nil {
		return TranslationSettings{}, "", fmt.Errorf("failed to read translation key: %w", err)
	}
	region, err := settingsStore.Get(translationRegionKey)
	if err != nil {
		return TranslationSettings{}, "", err
	}
	if region == "" {
		region = defaultTranslationRegion
	}
	language, err := settingsStore.Get(translationLanguageKey)
	if err != nil {
		return TranslationSettings{}, "", err
	}
	if !translate.IsSupportedLanguage(language) {
		language = defaultTranslationLanguage
	}
	return TranslationSettings{
		Configured: key != "",
		Region:     region,
		Language:   language,
		Languages:  translate.Languages,
	}, key, nil
}

func newTranslator(settingsStore *settings.Store, credStore *credentials.Store) (*translate.Client, error) {
	s, key, err := loadTranslationSettings(settingsStore, credStore)
	if err != nil {
		return nil, err
	}
	if key == "" {
		return nil, translate.ErrNotConfigured
	}
	return translate.NewClient(key, s.Region), nil
}

func checkTargetLanguage(to string) error {
	if !translate.IsSupportedLanguage(to) {
		return fmt.Errorf("unsupported language: %s", to)
	}
	return nil
}

// translateHTMLFragment translates HTML, sanitizing the result because it comes
// back from an outside service and is shown or inserted as HTML.
func translateHTMLFragment(ctx context.Context, client *translate.Client, html, to string) (string, error) {
	if err := checkTargetLanguage(to); err != nil {
		return "", err
	}
	result, err := client.Translate(ctx, html, to, true)
	if err != nil {
		return "", err
	}
	return sharedSanitizer.Sanitize(result.Text), nil
}

// ============================================================================
// Translation API - Exposed to frontend via Wails bindings
// ============================================================================

// GetTranslationSettings returns whether translation is set up, and its options.
func (a *App) GetTranslationSettings() (TranslationSettings, error) {
	s, _, err := loadTranslationSettings(a.settingsStore, a.credStore)
	return s, err
}

// SaveTranslationSettings saves the region and default language, and the key if
// one is given. An empty key keeps the saved one.
func (a *App) SaveTranslationSettings(key, region, language string) error {
	region = strings.ToLower(strings.TrimSpace(region))
	if region == "" {
		region = defaultTranslationRegion
	}
	if err := checkTargetLanguage(language); err != nil {
		return err
	}
	if key = strings.TrimSpace(key); key != "" {
		if err := a.credStore.SetExtensionSecret(translationSecretScope, translationSecretKey, key); err != nil {
			return fmt.Errorf("failed to save translation key: %w", err)
		}
	}
	if err := a.settingsStore.Set(translationRegionKey, region); err != nil {
		return err
	}
	return a.settingsStore.Set(translationLanguageKey, language)
}

// RemoveTranslationKey deletes the saved key, which turns translation off.
func (a *App) RemoveTranslationKey() error {
	return a.credStore.DeleteExtensionSecret(translationSecretScope, translationSecretKey)
}

// TestTranslation translates a short phrase to check the key and region work.
func (a *App) TestTranslation() (string, error) {
	client, err := newTranslator(a.settingsStore, a.credStore)
	if err != nil {
		return "", err
	}
	s, _, err := loadTranslationSettings(a.settingsStore, a.credStore)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(a.ctx, translationTimeout)
	defer cancel()
	result, err := client.Translate(ctx, "Thank you for your email.", s.Language, false)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

// TranslateMessage translates a stored email body. Encrypted mail is refused,
// so decrypted text never leaves the computer without the user meaning it to.
func (a *App) TranslateMessage(messageID, to string) (*TranslatedMessage, error) {
	if err := checkTargetLanguage(to); err != nil {
		return nil, err
	}
	msg, err := a.messageStore.Get(messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to load message: %w", err)
	}
	if msg == nil {
		return nil, fmt.Errorf("message not found")
	}
	if msg.SMIMEEncrypted || msg.PGPEncrypted || msg.HasSMIME || msg.HasPGP {
		return nil, errors.New("encrypted or signed messages aren't translated")
	}

	client, err := newTranslator(a.settingsStore, a.credStore)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, translationTimeout)
	defer cancel()

	// Keep the formatting when the HTML fits in one request
	if msg.BodyHTML != "" && utf8.RuneCountInString(msg.BodyHTML) <= translate.MaxRequestChars {
		result, err := client.Translate(ctx, msg.BodyHTML, to, true)
		if err != nil {
			return nil, err
		}
		return &TranslatedMessage{
			HTML:             sharedSanitizer.Sanitize(result.Text),
			DetectedLanguage: result.DetectedLanguage,
		}, nil
	}

	text := msg.BodyText
	if strings.TrimSpace(text) == "" && msg.BodyHTML != "" {
		text = email.ExtractPlainTextFromHTML(msg.BodyHTML)
	}
	result, err := client.TranslateLongText(ctx, text, to)
	if err != nil {
		return nil, err
	}
	return &TranslatedMessage{
		Text:             result.Text,
		AsText:           msg.BodyHTML != "",
		DetectedLanguage: result.DetectedLanguage,
	}, nil
}

// TranslateText translates plain text, such as a selection in an email.
func (a *App) TranslateText(text, to string) (translate.Result, error) {
	if err := checkTargetLanguage(to); err != nil {
		return translate.Result{}, err
	}
	client, err := newTranslator(a.settingsStore, a.credStore)
	if err != nil {
		return translate.Result{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, translationTimeout)
	defer cancel()
	return client.TranslateLongText(ctx, text, to)
}

// TranslateHTML translates a draft's HTML for the composer.
func (a *App) TranslateHTML(html, to string) (string, error) {
	client, err := newTranslator(a.settingsStore, a.credStore)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(a.ctx, translationTimeout)
	defer cancel()
	return translateHTMLFragment(ctx, client, html, to)
}

// GetTranslationSettings returns translation settings for the detached composer.
func (c *ComposerApp) GetTranslationSettings() (TranslationSettings, error) {
	s, _, err := loadTranslationSettings(c.settingsStore, c.credStore)
	return s, err
}

// TranslateHTML translates a draft's HTML in the detached composer.
func (c *ComposerApp) TranslateHTML(html, to string) (string, error) {
	client, err := newTranslator(c.settingsStore, c.credStore)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(c.ctx, translationTimeout)
	defer cancel()
	return translateHTMLFragment(ctx, client, html, to)
}
