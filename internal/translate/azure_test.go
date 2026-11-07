package translate

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type recorded struct {
	path, query, key, region, contentType string
	body                                  []map[string]string
}

// fakeAzure answers like Azure Translator, prefixing the target language to the
// text, and records each request.
func fakeAzure(t *testing.T, status int, reply string) (*Client, *[]recorded) {
	t.Helper()
	var requests []recorded
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		rec := recorded{
			path:        r.URL.Path,
			query:       r.URL.RawQuery,
			key:         r.Header.Get("Ocp-Apim-Subscription-Key"),
			region:      r.Header.Get("Ocp-Apim-Subscription-Region"),
			contentType: r.Header.Get("Content-Type"),
		}
		_ = json.Unmarshal(raw, &rec.body)
		requests = append(requests, rec)

		w.WriteHeader(status)
		if reply != "" {
			_, _ = w.Write([]byte(reply))
			return
		}
		text := ""
		if len(rec.body) > 0 {
			text = rec.body[0]["Text"]
		}
		to := r.URL.Query().Get("to")
		resp := []map[string]any{{
			"detectedLanguage": map[string]any{"language": "fr", "score": 1.0},
			"translations":     []map[string]any{{"text": to + ":" + strings.TrimSpace(text), "to": to}},
		}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(server.Close)

	client := NewClient(" secret-key ", "global")
	client.Endpoint = server.URL
	return client, &requests
}

func TestTranslateSendsAzureRequest(t *testing.T) {
	client, requests := fakeAzure(t, http.StatusOK, "")

	result, err := client.Translate(context.Background(), "Bonjour", "en", false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "en:Bonjour" || result.DetectedLanguage != "fr" {
		t.Errorf("got %+v", result)
	}

	r := (*requests)[0]
	if r.path != "/translate" || !strings.Contains(r.query, "api-version=3.0") || !strings.Contains(r.query, "to=en") {
		t.Errorf("unexpected request %s?%s", r.path, r.query)
	}
	if strings.Contains(r.query, "textType") {
		t.Error("plain text should not ask for HTML handling")
	}
	if r.key != "secret-key" {
		t.Errorf("key header = %q, want it trimmed", r.key)
	}
	if r.region != "" {
		t.Errorf("a global resource should send no region header, got %q", r.region)
	}
	if !strings.HasPrefix(r.contentType, "application/json") {
		t.Errorf("content type = %q", r.contentType)
	}
	if len(r.body) != 1 || r.body[0]["Text"] != "Bonjour" {
		t.Errorf("body = %+v", r.body)
	}
}

func TestTranslateHTMLAndRegion(t *testing.T) {
	client, requests := fakeAzure(t, http.StatusOK, "")
	client.Region = "northeurope"

	if _, err := client.Translate(context.Background(), "<p>Hallo</p>", "fr", true); err != nil {
		t.Fatal(err)
	}
	r := (*requests)[0]
	if !strings.Contains(r.query, "textType=html") {
		t.Errorf("query %q should ask for HTML handling", r.query)
	}
	if r.region != "northeurope" {
		t.Errorf("region header = %q", r.region)
	}
}

func TestTranslateRefusesBeforeSending(t *testing.T) {
	client, requests := fakeAzure(t, http.StatusOK, "")

	if _, err := client.Translate(context.Background(), "   ", "en", false); !errors.Is(err, ErrEmpty) {
		t.Errorf("blank text: got %v", err)
	}
	if _, err := client.Translate(context.Background(), strings.Repeat("é", MaxRequestChars+1), "en", false); !errors.Is(err, ErrTooLong) {
		t.Errorf("long text: got %v", err)
	}
	if _, err := NewClient("", "global").Translate(context.Background(), "x", "en", false); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("no key: got %v", err)
	}
	if len(*requests) != 0 {
		t.Errorf("%d requests were sent, want none", len(*requests))
	}
}

func TestTranslateAllowsExactlyTheLimit(t *testing.T) {
	client, _ := fakeAzure(t, http.StatusOK, "")
	if _, err := client.Translate(context.Background(), strings.Repeat("é", MaxRequestChars), "en", false); err != nil {
		t.Errorf("got %v, want the limit itself to be allowed", err)
	}
}

func TestTranslateErrors(t *testing.T) {
	tests := []struct {
		status int
		reply  string
		want   string
	}{
		{http.StatusUnauthorized, `{"error":{"code":401000,"message":"bad key"}}`, "rejected the key"},
		{http.StatusForbidden, `{"error":{"code":403001,"message":"quota exceeded"}}`, "quota exceeded"},
		{http.StatusTooManyRequests, `{"error":{"code":429000,"message":"slow down"}}`, "too many"},
		{http.StatusBadRequest, `{"error":{"code":400036,"message":"The target language is not valid."}}`, "target language is not valid"},
		{http.StatusInternalServerError, `not json`, "(500)"},
		{http.StatusOK, `[]`, "no translation"},
		{http.StatusOK, `garbage`, "unexpected reply"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			client, _ := fakeAzure(t, tt.status, tt.reply)
			_, err := client.Translate(context.Background(), "Bonjour", "en", false)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("got %v, want an error containing %q", err, tt.want)
			}
		})
	}
}

func TestSplitText(t *testing.T) {
	text := "First paragraph here.\n\nSecond paragraph is a bit longer.\nWith a second line.\n\nThird."
	for _, max := range []int{10, 25, 40, 1000} {
		chunks := SplitText(text, max)
		if strings.Join(chunks, "") != text {
			t.Errorf("max %d: chunks don't rejoin to the original", max)
		}
		for _, c := range chunks {
			if len([]rune(c)) > max {
				t.Errorf("max %d: chunk of %d runes: %q", max, len([]rune(c)), c)
			}
		}
	}

	chunks := SplitText(text, 40)
	if !strings.HasSuffix(chunks[0], "\n\n") {
		t.Errorf("first chunk %q should end at a paragraph break", chunks[0])
	}

	if got := SplitText(strings.Repeat("x", 25), 10); strings.Join(got, "") != strings.Repeat("x", 25) || len(got) != 3 {
		t.Errorf("unbreakable text: got %q", got)
	}
	if got := SplitText("", 10); got != nil {
		t.Errorf("empty text: got %q", got)
	}
	if got := SplitText("éééééé", 4); strings.Join(got, "") != "éééééé" || len(got) != 2 {
		t.Errorf("multi-byte text: got %q", got)
	}
}

func TestTranslateLongTextKeepsBreaks(t *testing.T) {
	client, requests := fakeAzure(t, http.StatusOK, "")

	para := strings.Repeat("word ", 5000) // 25,000 characters
	text := para + "\n\n" + para + "\n\n" + "end"

	result, err := client.TranslateLongText(context.Background(), text, "de")
	if err != nil {
		t.Fatal(err)
	}
	if len(*requests) < 2 {
		t.Errorf("sent %d requests, want the text split", len(*requests))
	}
	for i, r := range *requests {
		if n := len([]rune(r.body[0]["Text"])); n > MaxRequestChars {
			t.Errorf("request %d had %d characters", i, n)
		}
	}
	if strings.Count(result.Text, "\n\n") != 2 {
		t.Errorf("paragraph breaks were lost: %d found", strings.Count(result.Text, "\n\n"))
	}
	if result.DetectedLanguage != "fr" {
		t.Errorf("detected = %q", result.DetectedLanguage)
	}
}

func TestIsSupportedLanguage(t *testing.T) {
	for _, code := range []string{"en", "fr", "de", "es"} {
		if !IsSupportedLanguage(code) {
			t.Errorf("%s should be supported", code)
		}
	}
	if IsSupportedLanguage("en-GB") || IsSupportedLanguage("") {
		t.Error("unexpected language accepted")
	}
}
