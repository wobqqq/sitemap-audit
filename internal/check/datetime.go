// Package check validates sitemap entries and crawled pages.
package check

import (
	"errors"
	"regexp"
	"time"
)

var w3c = regexp.MustCompile(`^\d{4}(-\d{2}(-\d{2}(T\d{2}:\d{2}(:\d{2}(\.\d+)?)?(Z|[+-]\d{2}:\d{2}))?)?)?$`)

var w3cLayouts = []string{
	"2006",
	"2006-01",
	"2006-01-02",
	"2006-01-02T15:04Z07:00",
	"2006-01-02T15:04:05Z07:00",
	time.RFC3339Nano,
}

// ParseW3C reads a W3C datetime as the sitemap protocol defines it.
func ParseW3C(v string) (time.Time, error) {
	if !w3c.MatchString(v) {
		return time.Time{}, errors.New("not a W3C datetime")
	}
	for _, l := range w3cLayouts {
		if t, err := time.Parse(l, v); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("not a valid date")
}
