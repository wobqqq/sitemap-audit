// Package langcode validates hreflang values.
package langcode

import (
	"errors"
	"fmt"
	"strings"
)

var (
	languageSet = toSet(languages)
	regionSet   = toSet(regions)
)

func toSet(list string) map[string]bool {
	f := strings.Fields(list)
	m := make(map[string]bool, len(f))
	for _, v := range f {
		m[v] = true
	}
	return m
}

// Check validates an hreflang value: x-default or language[-Script][-REGION].
func Check(tag string) error {
	if strings.EqualFold(tag, "x-default") {
		return nil
	}
	if tag == "" {
		return errors.New("empty hreflang")
	}
	parts := strings.Split(strings.ReplaceAll(tag, "_", "-"), "-")
	if strings.Contains(tag, "_") {
		return fmt.Errorf("%q uses _ instead of -", tag)
	}
	lang := strings.ToLower(parts[0])
	if !languageSet[lang] {
		return fmt.Errorf("%q: %q is not an ISO 639-1 language code", tag, parts[0])
	}
	rest := parts[1:]
	if len(rest) > 0 && len(rest[0]) == 4 && isAlpha(rest[0]) {
		rest = rest[1:]
	}
	switch len(rest) {
	case 0:
		return nil
	case 1:
		r := rest[0]
		if len(r) == 3 && isDigits(r) {
			return nil
		}
		if len(r) == 2 && regionSet[strings.ToUpper(r)] {
			return nil
		}
		if strings.EqualFold(r, "UK") {
			return fmt.Errorf("%q: the region code of the United Kingdom is GB", tag)
		}
		return fmt.Errorf("%q: %q is not an ISO 3166-1 region code", tag, r)
	default:
		return fmt.Errorf("%q has too many subtags", tag)
	}
}

func isAlpha(s string) bool {
	for _, c := range s {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') {
			return false
		}
	}
	return true
}

func isDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
