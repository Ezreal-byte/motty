package i18n

import (
	"reflect"
	"strings"
	"testing"
	"unicode"
)

// A language that forgot a string would paint blank labels. Reflection is fine
// here: it runs in tests, never on a paint path.
func TestEveryLanguageFillsEveryMessage(t *testing.T) {
	for _, language := range []Language{English, Chinese} {
		messages := reflect.ValueOf(For(language))
		for index := 0; index < messages.NumField(); index++ {
			if strings.TrimSpace(messages.Field(index).String()) == "" {
				t.Errorf("%s: %s is empty", language, messages.Type().Field(index).Name)
			}
		}
	}
}

// The Chinese catalog must not silently fall back to English text for the
// strings a user reads constantly, and format strings must keep their verbs.
func TestChineseCatalogIsTranslated(t *testing.T) {
	english, chinese := reflect.ValueOf(For(English)), reflect.ValueOf(For(Chinese))
	for index := 0; index < english.NumField(); index++ {
		name := english.Type().Field(index).Name
		en, zh := english.Field(index).String(), chinese.Field(index).String()
		if strings.Count(en, "%") != strings.Count(zh, "%") {
			t.Errorf("%s: verbs differ between %q and %q", name, en, zh)
		}
		if strings.HasSuffix(name, "Fmt") && !hasHan(zh) {
			t.Errorf("%s: %q has no Chinese text", name, zh)
		}
	}
}

func hasHan(value string) bool {
	for _, r := range value {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
