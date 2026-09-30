package workbench

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/miroslav-matejovsky/inspector/harness/inspected"
	"github.com/miroslav-matejovsky/inspector/source"
)

// Config configures the workbench. Every field is required and has no default.
type Config struct {
	Addr      string     // listen address, for example "localhost:8080"
	LogDir    string     // directory of the log files, used by cmd/workbench
	LogLevel  slog.Level // lowest level written to the log file, used by cmd/workbench
	Inspected inspected.Config
	Source    SourceConfig
}

// SourceConfig configures the Source of the workbench. Every field is required.
type SourceConfig struct {
	DatabasePath string        // SQLite file of the collected signals
	Interval     time.Duration // time between collection rounds
	Timeout      time.Duration // max wait for one read of one target
	Retention    time.Duration // signals older than this are deleted
	MaxBodyBytes int64         // a larger body is stored as a failed read
	Targets      []SourceTarget
}

// SourceTarget is one path of the workbench server that the Source reads.
type SourceTarget struct {
	Name string // source.Target.Name
	Path string // absolute path, for example /inspected/health/ready
}

// sourceConfig builds the source.Config that reads every target at baseURL + Path.
func (c SourceConfig) sourceConfig(baseURL string) source.Config {
	targets := make([]source.Target, len(c.Targets))
	for i, t := range c.Targets {
		targets[i] = source.Target{Name: t.Name, URL: baseURL + t.Path}
	}
	return source.Config{
		Targets:      targets,
		Interval:     c.Interval,
		Timeout:      c.Timeout,
		Retention:    c.Retention,
		MaxBodyBytes: c.MaxBodyBytes,
		DatabasePath: c.DatabasePath,
	}
}

// Validate reports whether the config is usable.
func (c Config) Validate() error {
	if c.Addr == "" {
		return errors.New("workbench: listen address is required")
	}
	if c.LogDir == "" {
		return errors.New("workbench: log dir is required")
	}
	if err := c.Inspected.Validate(); err != nil {
		return err
	}
	for i, t := range c.Source.Targets {
		if !strings.HasPrefix(t.Path, "/") {
			return fmt.Errorf("workbench: source target %d: path %q must start with /", i, t.Path)
		}
	}
	// Run builds the real URLs from the listener address, because Addr may
	// end in ":0"; Addr is good enough to validate everything else up front.
	if err := c.Source.sourceConfig("http://" + c.Addr).Validate(); err != nil {
		return fmt.Errorf("workbench: %w", err)
	}
	return nil
}

// targetsFlag collects repeated -source-target name=path values in order.
type targetsFlag []SourceTarget

// String returns the targets as name=path pairs separated by commas. The flag
// package calls it on a nil receiver to print defaults.
func (f *targetsFlag) String() string {
	if f == nil {
		return ""
	}
	pairs := make([]string, len(*f))
	for i, t := range *f {
		pairs[i] = t.Name + "=" + t.Path
	}
	return strings.Join(pairs, ",")
}

// Set appends one name=path target.
func (f *targetsFlag) Set(value string) error {
	name, path, ok := strings.Cut(value, "=")
	if !ok {
		return fmt.Errorf("want name=path, got %q", value)
	}
	*f = append(*f, SourceTarget{Name: name, Path: path})
	return nil
}

// ParseConfig parses command-line arguments, without the program name. Every
// flag is required: a missing flag is an error even when its zero value would
// be valid. Usage and parse errors are written to output.
func ParseConfig(args []string, output io.Writer) (Config, error) {
	var cfg Config
	fs := flag.NewFlagSet("workbench", flag.ContinueOnError)
	fs.SetOutput(output)
	fs.StringVar(&cfg.Addr, "addr", "", "listen address, for example localhost:8080 (required)")
	fs.StringVar(&cfg.LogDir, "log-dir", "",
		"directory of the log files, created when missing, for example logs (required)")
	fs.TextVar(&cfg.LogLevel, "log-level", slog.LevelInfo,
		"lowest level written to the log file: debug, info, warn or error (required)")
	fs.Uint64Var(&cfg.Inspected.Seed, "inspected-seed", 0,
		"seed of the inspected traffic generator (required)")
	fs.IntVar(&cfg.Inspected.OrdersPerTick, "inspected-orders-per-tick", 0,
		"simulated orders placed per tick, 0..100 (required)")
	fs.DurationVar(&cfg.Inspected.TickInterval, "inspected-tick-interval", 0,
		"wall-clock time between simulation ticks, for example 1s (required)")
	fs.DurationVar(&cfg.Inspected.RequestTimeout, "inspected-request-timeout", 0,
		"max wait for the simulation per HTTP request, for example 2s (required)")
	fs.StringVar(&cfg.Source.DatabasePath, "source-database", "",
		"SQLite file of the collected signals, for example data/source.db (required)")
	fs.DurationVar(&cfg.Source.Interval, "source-interval", 0,
		"time between collection rounds, for example 5s (required)")
	fs.DurationVar(&cfg.Source.Timeout, "source-timeout", 0,
		"max wait for one read of one target, for example 2s (required)")
	fs.DurationVar(&cfg.Source.Retention, "source-retention", 0,
		"signals older than this are deleted, for example 10m (required)")
	fs.Int64Var(&cfg.Source.MaxBodyBytes, "source-max-body-bytes", 0,
		"a larger body is stored as a failed read, for example 2097152 (required)")
	fs.Var((*targetsFlag)(&cfg.Source.Targets), "source-target",
		"name=path of a workbench path to read, repeated (required)")

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
