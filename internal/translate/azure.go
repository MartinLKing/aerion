// Package translate is a small client for Azure Translator (Text Translation v3).
// Text is only sent when the user asks for a translation.
package translate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// DefaultEndpoint is Azure Translator's global endpoint.
const DefaultEndpoint = "https://api.cognitive.microsofttranslator.com"

// MaxRequestChars is Azure's limit for one translate request, counting HTML tags.
const MaxRequestChars = 50000

// chunkChars leaves headroom under MaxRequestChars when splitting long text.
const chunkChars = 45000

// Languages lists the target languages offered in the app.
var Languages = []string{"en", "fr", "de", "es"}

var (
	ErrNotConfigured = errors.New("translation isn't set up: add your Azure Translator key in Settings")
	ErrTooLong       = errors.New("this text is too long to translate in one go")
	ErrEmpty         = errors.New("there's no text to translate")
)

// Client calls Azure Translator.
type Client struct {
	Key string
	// Region is the resource's location, such as "northeurope". "global" or ""
	// means a global resource, which doesn't send a region header.
	Region   string
	Endpoint string
	HTTP     *http.Client
}

// NewClient returns a client for the global endpoint.
func NewClient(key, region string) *Client {
	return &Client{
		Key:      strings.TrimSpace(key),
		Region:   strings.TrimSpace(region),
		Endpoint: DefaultEndpoint,
		HTTP:     &http.Client{Timeout: 30 * time.Second},
	}
}

// Result is a translation and the language Azure detected in the original.
type Result struct {
	Text             string `json:"text"`
	DetectedLanguage string `json:"detectedLanguage"`
}

// APIError is an error response from Azure, with a message the user can act on.
type APIError struct {
	Status  int
	Code    int
	Message string
}

func (e *APIError) Error() string {
	switch e.Status {
	case http.StatusUnauthorized:
		return "Azure rejected the key: check the key and region in Settings"
	case http.StatusForbidden:
		return "Azure refused the request, possibly because this month's free allowance is used up: " + e.Message
	case http.StatusTooManyRequests:
		return "too many translations in a short time: wait a minute and try again"
	}
	if e.Message != "" {
		return fmt.Sprintf("translation failed (%d): %s", e.Status, e.Message)
	}
	return fmt.Sprintf("translation failed (%d)", e.Status)
}

// IsSupportedLanguage reports whether code is one of the offered target languages.
func IsSupportedLanguage(code string) bool {
	for _, l := range Languages {
		if l == code {
			return true
		}
	}
	return false
}

// Translate translates text, which is HTML when html is true, into the language
// to. Text over MaxRequestChars returns ErrTooLong.
func (c *Client) Translate(ctx context.Context, text, to string, html bool) (Result, error) {
	if c == nil || c.Key == "" {
		return Result{}, ErrNotConfigured
	}
	if strings.TrimSpace(text) == "" {
		return Result{}, ErrEmpty
	}
	if utf8.RuneCountInString(text) > MaxRequestChars {
		return Result{}, ErrTooLong
	}

	query := url.Values{}
	query.Set("api-version", "3.0")
	query.Set("to", to)
	if html {
		query.Set("textType", "html")
	}
	endpoint := strings.TrimRight(c.Endpoint, "/")
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}

	body, err := json.Marshal([]map[string]string{{"Text": text}})
	if err != nil {
		return Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/translate?"+query.Encode(), bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("Ocp-Apim-Subscription-Key", c.Key)
	if region := strings.ToLower(c.Region); region != "" && region != "global" {
		req.Header.Set("Ocp-Apim-Subscription-Region", c.Region)
	}

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("couldn't reach Azure Translator: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return Result{}, fmt.Errorf("failed to read translation: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		apiErr := &APIError{Status: resp.StatusCode}
		var parsed struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(respBody, &parsed) == nil {
			apiErr.Code = parsed.Error.Code
			apiErr.Message = parsed.Error.Message
		}
		return Result{}, apiErr
	}

	var parsed []struct {
		DetectedLanguage struct {
			Language string `json:"language"`
		} `json:"detectedLanguage"`
		Translations []struct {
			Text string `json:"text"`
		} `json:"translations"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return Result{}, fmt.Errorf("unexpected reply from Azure Translator: %w", err)
	}
	if len(parsed) == 0 || len(parsed[0].Translations) == 0 {
		return Result{}, errors.New("Azure Translator returned no translation")
	}
	return Result{
		Text:             parsed[0].Translations[0].Text,
		DetectedLanguage: parsed[0].DetectedLanguage.Language,
	}, nil
}

// TranslateLongText translates plain text of any length, splitting it into
// pieces at paragraph or line breaks so each request stays under the limit.
func (c *Client) TranslateLongText(ctx context.Context, text, to string) (Result, error) {
	chunks := SplitText(text, chunkChars)
	if len(chunks) == 0 {
		return Result{}, ErrEmpty
	}

	var out strings.Builder
	detected := ""
	for _, chunk := range chunks {
		if strings.TrimSpace(chunk) == "" {
			out.WriteString(chunk)
			continue
		}
		result, err := c.Translate(ctx, chunk, to, false)
		if err != nil {
			return Result{}, err
		}
		if detected == "" {
			detected = result.DetectedLanguage
		}
		out.WriteString(result.Text)
		// Azure drops trailing whitespace, which held the break between chunks
		out.WriteString(trailingWhitespace(chunk))
	}
	return Result{Text: out.String(), DetectedLanguage: detected}, nil
}

// SplitText splits text into pieces of at most maxRunes characters, preferring
// paragraph breaks, then line breaks, then spaces. Joining the pieces gives the
// original text back.
func SplitText(text string, maxRunes int) []string {
	if text == "" {
		return nil
	}
	var chunks []string
	for utf8.RuneCountInString(text) > maxRunes {
		limit := byteOffsetOfRune(text, maxRunes)
		window := text[:limit]
		cut := -1
		for _, sep := range []string{"\n\n", "\n", " "} {
			if i := strings.LastIndex(window, sep); i > 0 {
				cut = i + len(sep)
				break
			}
		}
		if cut <= 0 {
			cut = limit
		}
		chunks = append(chunks, text[:cut])
		text = text[cut:]
	}
	if text != "" {
		chunks = append(chunks, text)
	}
	return chunks
}

func byteOffsetOfRune(s string, n int) int {
	count := 0
	for i := range s {
		if count == n {
			return i
		}
		count++
	}
	return len(s)
}

func trailingWhitespace(s string) string {
	return s[len(strings.TrimRight(s, " \t\r\n")):]
}
