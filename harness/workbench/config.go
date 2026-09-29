package workbench

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/miroslav-matejovsky/inspector/harness/inspected"
)

// Config configures the workbench. Every field is required and has no default.
type Config struct {
	Addr                   string // listen address, for example "localhost:8080"
	Inspected              inspected.Config
	InspectorSourceTimeout time.Duration // max wait for one read of the inspected service by the inspector, must be positive
}

// Validate reports whether the config is usable.
func (c Config) Validate() error {
	if c.Addr == "" {
		return errors.New("workbench: listen address is required")
	}
	if c.InspectorSourceTimeout <= 0 {
		return fmt.Errorf("workbench: inspector source timeout must be positive, got %s", c.InspectorSourceTimeout)
	}
	return c.Inspected.Validate()
}

// ParseConfig parses command-line arguments, without the program name. Every
// flag is required: a missing flag is an error even when its zero value would
// be valid. Usage and parse errors are written to output.
func ParseConfig(args []string, output io.Writer) (Config, error) {
	var cfg Config
	fs := flag.NewFlagSet("workbench", flag.ContinueOnError)
	fs.SetOutput(output)
	fs.StringVar(&cfg.Addr, "addr", "", "listen address, for example localhost:8080 (required)")
	fs.Uint64Var(&cfg.Inspected.Seed, "inspected-seed", 0,
		"seed of the inspected traffic generator (required)")
	fs.IntVar(&cfg.Inspected.OrdersPerTick, "inspected-orders-per-tick", 0,
		"simulated orders placed per tick, 0..100 (required)")
	fs.DurationVar(&cfg.Inspected.TickInterval, "inspected-tick-interval", 0,
		"wall-clock time between simulation ticks, for example 1s (required)")
	fs.DurationVar(&cfg.Inspected.RequestTimeout, "inspected-request-timeout", 0,
		"max wait for the simulation per HTTP request, for example 2s (required)")
	fs.DurationVar(&cfg.InspectorSourceTimeout, "inspector-source-timeout", 0,
		"max wait for one read of the inspected service by the inspector, for example 2s (required)")

	if err := fs.Parse(args); err != nil {
		return Config{}, fmt.Errorf("workbench: parse flags: %w", err)
	}

	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	var missing []string
	fs.VisitAll(func(f *flag.Flag) {
		if !set[f.Name] {
			missing = append(missing, "-"+f.Name)
		}
	})
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("workbench: missing required flags: %s", strings.Join(missing, ", "))
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
