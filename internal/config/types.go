package config

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// Duration is a time.Duration read from "1.5s", "300ms" or a number of seconds.
type Duration time.Duration

// D returns the value as a time.Duration.
func (d Duration) D() time.Duration { return time.Duration(d) }

// UnmarshalYAML reads a duration string or a number of seconds.
func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	v, err := parseDuration(n.Value)
	if err != nil {
		return fmt.Errorf("line %d: %w", n.Line, err)
	}
	*d = Duration(v)
	return nil
}

// MarshalYAML writes the duration as a string.
func (d Duration) MarshalYAML() (any, error) {
	return time.Duration(d).String(), nil
}

func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		if f < 0 {
			return 0, fmt.Errorf("negative duration %q", s)
		}
		return time.Duration(f * float64(time.Second)), nil
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q (use e.g. 15s, 500ms, 2m)", s)
	}
	if v < 0 {
		return 0, fmt.Errorf("negative duration %q", s)
	}
	return v, nil
}

// Range is a random pause between Min and Max.
type Range struct {
	Min Duration `yaml:"min"`
	Max Duration `yaml:"max"`
}

// Pick returns a random duration inside the range.
func (r Range) Pick() time.Duration {
	lo, hi := r.Min.D(), r.Max.D()
	if hi <= lo {
		return lo
	}
	return lo + rand.N(hi-lo+1) //nolint:gosec // jitter only
}

// Size is a byte count read from 1048576, "512KB", "5MB" or "1GiB".
type Size int64

// UnmarshalYAML reads a size.
func (s *Size) UnmarshalYAML(n *yaml.Node) error {
	v, err := ParseSize(n.Value)
	if err != nil {
		return fmt.Errorf("line %d: %w", n.Line, err)
	}
	*s = Size(v)
	return nil
}

// ParseSize reads a byte count with an optional KB/MB/GB (or KiB/MiB/GiB) suffix.
func ParseSize(v string) (int64, error) {
	s := strings.ToUpper(strings.TrimSpace(v))
	if s == "" {
		return 0, nil
	}
	mult := int64(1)
	for _, u := range []struct {
		suffix string
		mult   int64
	}{
		{"KIB", 1 << 10}, {"MIB", 1 << 20}, {"GIB", 1 << 30},
		{"KB", 1000}, {"MB", 1000 * 1000}, {"GB", 1000 * 1000 * 1000},
		{"B", 1},
	} {
		if strings.HasSuffix(s, u.suffix) {
			mult = u.mult
			s = strings.TrimSpace(strings.TrimSuffix(s, u.suffix))
			break
		}
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 {
		return 0, fmt.Errorf("invalid size %q (use e.g. 5MB or 1048576)", v)
	}
	return int64(f * float64(mult)), nil
}

// Site is one audited site or section.
type Site struct {
	URL         string `yaml:"url"`
	Concurrency int    `yaml:"concurrency,omitempty"`
}

// UnmarshalYAML accepts a plain URL or a mapping.
func (s *Site) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		s.URL = n.Value
		return nil
	}
	type plain Site
	var p plain
	if err := n.Decode(&p); err != nil {
		return err
	}
	*s = Site(p)
	return nil
}

// ID is the site URL without scheme, query and trailing slash.
func (s Site) ID() string { return NormalizeID(s.URL) }

// NormalizeID turns a URL or an id into the canonical site id.
func NormalizeID(v string) string {
	v = strings.TrimSpace(v)
	low := strings.ToLower(v)
	switch {
	case strings.HasPrefix(low, "https://"):
		v = v[len("https://"):]
	case strings.HasPrefix(low, "http://"):
		v = v[len("http://"):]
	}
	if i := strings.IndexAny(v, "?#"); i >= 0 {
		v = v[:i]
	}
	v = strings.TrimRight(v, "/")
	if i := strings.IndexByte(v, '/'); i >= 0 {
		return strings.ToLower(v[:i]) + v[i:]
	}
	return strings.ToLower(v)
}
