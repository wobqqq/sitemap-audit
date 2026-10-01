package langcode

import (
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	for _, ok := range []string{"x-default", "X-Default", "en", "EN", "en-US", "en-gb", "zh-Hant", "zh-Hant-TW", "es-419", "pt-BR", "sr-Latn-RS"} {
		if err := Check(ok); err != nil {
			t.Errorf("Check(%q) = %v", ok, err)
		}
	}
	for tag, want := range map[string]string{
		"":           "empty",
		"en_US":      "instead of -",
		"english":    "ISO 639-1",
		"xx":         "ISO 639-1",
		"en-UK":      "GB",
		"en-ZZ":      "ISO 3166-1",
		"en-USA":     "ISO 3166-1",
		"en-US-x-ab": "too many",
	} {
		err := Check(tag)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Check(%q) = %v, want an error with %q", tag, err, want)
		}
	}
}

func TestListsAreWellFormed(t *testing.T) {
	for code := range languageSet {
		if len(code) != 2 || strings.ToLower(code) != code {
			t.Errorf("language %q", code)
		}
	}
	for code := range regionSet {
		if len(code) != 2 || strings.ToUpper(code) != code {
			t.Errorf("region %q", code)
		}
	}
	if len(languageSet) < 180 || len(regionSet) < 240 {
		t.Errorf("lists look incomplete: %d languages, %d regions", len(languageSet), len(regionSet))
	}
}
