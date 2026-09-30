package source

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"time"
)

var validName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,62}$`)

// Target is one HTTP endpoint that a Source reads with GET.
type Target struct {
	Name string // unique name of the target, see Config.Validate
	URL  string // absolute http or https URL
}

// Config configures a Source. Every field is required and has no default.
type Config struct {
	Targets      []Target      // read in every round, at least one
	Interval     time.Duration // time between the starts of two rounds
	Timeout      time.Duration // max wait for one read of one target
	Retention    time.Duration // signals older than this are deleted after each round
	MaxBodyBytes int64         // a larger body is stored as a failed read
	DatabasePath string        // SQLite file, see package internal/signalstore
}

// Validate reports the first rule that c breaks, or nil. Target names must
// match ^[a-z0-9][a-z0-9_-]{0,62}$ and be unique; durations and the body limit
// must be positive.
func (c Config) Validate() error {
	if len(c.Targets) == 0 {
		return errors.New("source: at least one target is required")
	}
	seen := map[string]bool{}
	for i, t := range c.Targets {
		if !validName.MatchString(t.Name) {
			return fmt.Errorf("source: target %d: name %q must match %s", i, t.Name, validName)
		}
		if seen[t.Name] {
			return fmt.Errorf("source: target %d: duplicate name %q", i, t.Name)
		}
		seen[t.Name] = true
		u, err := url.Parse(t.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("source: target %d: url %q must be an absolute http or https URL", i, t.URL)
		}
	}
	if c.Interval <= 0 {
		return fmt.Errorf("source: interval must be positive, got %s", c.Interval)
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("source: timeout must be positive, got %s", c.Timeout)
	}
	if c.Retention <= 0 {
		return fmt.Errorf("source: retention must be positive, got %s", c.Retention)
	}
	if c.MaxBodyBytes <= 0 {
		return fmt.Errorf("source: max body bytes must be positive, got %d", c.MaxBodyBytes)
	}
	if c.DatabasePath == "" {
		return errors.New("source: database path is required")
	}
	return nil
}
