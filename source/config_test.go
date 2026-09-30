package source_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/source"
)

func baseConfig() source.Config {
	return source.Config{
		Targets: []source.Target{
			{Name: "alpha", URL: "http://example.test/a"},
			{Name: "beta", URL: "http://example.test/b"},
		},
		Interval:     time.Hour,
		Timeout:      time.Second,
		Retention:    time.Hour,
		MaxBodyBytes: 1024,
		DatabasePath: "s.db",
	}
}

func TestConfigValidateAccepts(t *testing.T) {
	tests := map[string]func(c *source.Config){
		"valid":         func(*source.Config) {},
		"63 characters": func(c *source.Config) { c.Targets[0].Name = "a" + strings.Repeat("0", 62) },
		"https":         func(c *source.Config) { c.Targets[0].URL = "https://example.test/a" },
		"separators":    func(c *source.Config) { c.Targets[0].Name = "a_b-c" },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			c := baseConfig()
			change(&c)

			require.NoError(t, c.Validate())
		})
	}
}

func TestConfigValidateRejects(t *testing.T) {
	tests := map[string]func(c *source.Config){
		"no targets":         func(c *source.Config) { c.Targets = nil },
		"empty name":         func(c *source.Config) { c.Targets[0].Name = "" },
		"upper case name":    func(c *source.Config) { c.Targets[0].Name = "Alpha" },
		"leading dash":       func(c *source.Config) { c.Targets[0].Name = "-a" },
		"64 characters":      func(c *source.Config) { c.Targets[0].Name = "a" + strings.Repeat("0", 63) },
		"duplicate name":     func(c *source.Config) { c.Targets[1].Name = "alpha" },
		"empty url":          func(c *source.Config) { c.Targets[0].URL = "" },
		"ftp url":            func(c *source.Config) { c.Targets[0].URL = "ftp://x/a" },
		"no host":            func(c *source.Config) { c.Targets[0].URL = "http:///a" },
		"relative url":       func(c *source.Config) { c.Targets[0].URL = "/a" },
		"unparsable url":     func(c *source.Config) { c.Targets[0].URL = "::" },
		"zero interval":      func(c *source.Config) { c.Interval = 0 },
		"negative interval":  func(c *source.Config) { c.Interval = -time.Second },
		"zero timeout":       func(c *source.Config) { c.Timeout = 0 },
		"negative timeout":   func(c *source.Config) { c.Timeout = -time.Second },
		"zero retention":     func(c *source.Config) { c.Retention = 0 },
		"negative retention": func(c *source.Config) { c.Retention = -time.Second },
		"zero body limit":    func(c *source.Config) { c.MaxBodyBytes = 0 },
		"empty database":     func(c *source.Config) { c.DatabasePath = "" },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			c := baseConfig()
			change(&c)

			require.Error(t, c.Validate())
		})
	}
}

func TestConfigValidateNamesTarget(t *testing.T) {
	c := baseConfig()
	c.Targets[1].URL = ""

	require.ErrorContains(t, c.Validate(), "target 1")
}
