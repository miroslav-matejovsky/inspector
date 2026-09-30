package source

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/miroslav-matejovsky/inspector/internal/signalstore"
)

// Signal is one read of one target.
type Signal struct {
	Target      string        // Target.Name
	URL         string        // Target.URL
	ObservedAt  time.Time     // start of the read, UTC
	Duration    time.Duration // from the start of the read until the body was read or the read failed
	StatusCode  int           // HTTP status; 0 when no response was received
	ContentType string        // Content-Type header; empty when no response was received
	Body        []byte        // response body; nil when Error is set
	Error       string        // why the read failed; empty when a body within MaxBodyBytes was read
}

// TargetSummary is what is stored for one configured target.
type TargetSummary struct {
	Target  Target
	Signals int64   // number of stored signals of the target
	Latest  *Signal // the signal stored last; nil when none is stored
}

// Source collects signals from the targets of its Config.
type Source struct {
	cfg     Config
	client  *http.Client
	logger  *slog.Logger
	store   *signalstore.Store
	now     func() time.Time // time.Now; replaced in tests by SetClock
	mu      sync.Mutex       // serializes rounds
	failing map[string]bool  // target name -> the last read failed; guarded by mu
}

// Open validates cfg and opens the database at cfg.DatabasePath. client
// sends the requests; each request is bounded by cfg.Timeout, so client
// needs no timeout of its own. logger receives start and target failure and
// recovery records. Close releases the database.
func Open(ctx context.Context, cfg Config, client *http.Client, logger *slog.Logger) (*Source, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if client == nil {
		return nil, errors.New("source: http client is required")
	}
	if logger == nil {
		return nil, errors.New("source: logger is required")
	}
	store, err := signalstore.Open(ctx, cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	return &Source{
		cfg:     cfg,
		client:  client,
		logger:  logger,
		store:   store,
		now:     time.Now,
		failing: map[string]bool{},
	}, nil
}

// Collect reads every target once, concurrently, and stores one signal per
// target in one transaction. Then it deletes signals observed before the
// start of the round minus Retention. A failed read is a stored signal, not
// an error. Collect returns an error when ctx ends during the round, which is
// then dropped, or when the store fails. Rounds do not overlap: a second call
// waits for the first.
func (s *Source) Collect(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	start := s.now()
	signals := make([]Signal, len(s.cfg.Targets))
	var wg sync.WaitGroup
	for i, t := range s.cfg.Targets {
		wg.Go(func() { signals[i] = s.read(ctx, t) })
	}
	wg.Wait()
	// Reads cut short by the end of ctx say nothing about the targets: the
	// round is dropped, neither logged nor stored.
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("source: round interrupted: %w", err)
	}
	s.logTransitions(signals)

	records := make([]signalstore.Record, len(signals))
	for i, sig := range signals {
		records[i] = toRecord(sig)
	}
	if err := s.store.Insert(ctx, records); err != nil {
		return fmt.Errorf("source: %w", err)
	}
	if _, err := s.store.DeleteBefore(ctx, start.Add(-s.cfg.Retention)); err != nil {
		return fmt.Errorf("source: %w", err)
	}
	return nil
}

// read reads one target. Any response, whatever its status code, is a
// successful read; only a missing response, an unreadable body or a body
// larger than MaxBodyBytes sets Error.
func (s *Source) read(ctx context.Context, t Target) (sig Signal) {
	start := s.now()
	sig = Signal{Target: t.Name, URL: t.URL, ObservedAt: start.UTC()}
	// sig is the named result, so the duration is set on every return path.
	defer func() { sig.Duration = s.now().Sub(start) }()

	ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.URL, nil)
	if err != nil {
		sig.Error = err.Error()
		return sig
	}
	resp, err := s.client.Do(req)
	if err != nil {
		sig.Error = err.Error()
		return sig
	}
	sig.StatusCode = resp.StatusCode
	sig.ContentType = resp.Header.Get("Content-Type")
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, s.cfg.MaxBodyBytes+1))
	closeErr := resp.Body.Close()
	switch {
	case readErr != nil:
		sig.Error = "read body: " + readErr.Error()
	case int64(len(body)) > s.cfg.MaxBodyBytes:
		sig.Error = fmt.Sprintf("body exceeds %d bytes", s.cfg.MaxBodyBytes)
	case closeErr != nil:
		sig.Error = "close body: " + closeErr.Error()
	default:
		sig.Body = body
	}
	return sig
}

// logTransitions logs a target once when it starts failing and once when it
// recovers. A target not read yet counts as not failing. Called with mu held.
func (s *Source) logTransitions(signals []Signal) {
	for _, sig := range signals {
		failing := sig.Error != ""
		if failing == s.failing[sig.Target] {
			continue
		}
		s.failing[sig.Target] = failing
		if failing {
			s.logger.Warn("source target failing", "target", sig.Target, "url", sig.URL, "error", sig.Error)
		} else {
			s.logger.Info("source target recovered", "target", sig.Target, "url", sig.URL)
		}
	}
}

// Run collects one round at once and then one round per Interval until ctx
// ends. Ticks that arrive during a round are dropped (time.Ticker). Run
// returns nil when ctx ends and the error of Collect when the store fails.
func (s *Source) Run(ctx context.Context) error {
	s.logger.Info("source started",
		"targets", len(s.cfg.Targets), "interval", s.cfg.Interval, "database", s.cfg.DatabasePath)
	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()
	for {
		if err := s.Collect(ctx); err != nil {
			if ctx.Err() != nil {
				// The round was interrupted by the end of ctx.
				return nil
			}
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// Summary returns one entry per configured target, in the order of
// Config.Targets. Signals stored under names that are not configured, for
// example by an earlier run with other targets, are left out.
func (s *Source) Summary(ctx context.Context) ([]TargetSummary, error) {
	stats, err := s.store.Stats(ctx)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	byTarget := make(map[string]signalstore.Stats, len(stats))
	for _, st := range stats {
		byTarget[st.Target] = st
	}
	out := make([]TargetSummary, len(s.cfg.Targets))
	for i, t := range s.cfg.Targets {
		out[i].Target = t
		if st, ok := byTarget[t.Name]; ok {
			latest := fromRecord(st.Latest)
			out[i].Signals = st.Count
			out[i].Latest = &latest
		}
	}
	return out, nil
}

// Close closes the database. Call it after Run has returned.
func (s *Source) Close() error {
	if err := s.store.Close(); err != nil {
		return fmt.Errorf("source: %w", err)
	}
	return nil
}

func toRecord(s Signal) signalstore.Record {
	return signalstore.Record{
		Target:      s.Target,
		URL:         s.URL,
		ObservedAt:  s.ObservedAt,
		Duration:    s.Duration,
		StatusCode:  s.StatusCode,
		ContentType: s.ContentType,
		Body:        s.Body,
		Error:       s.Error,
	}
}

func fromRecord(r signalstore.Record) Signal {
	return Signal{
		Target:      r.Target,
		URL:         r.URL,
		ObservedAt:  r.ObservedAt,
		Duration:    r.Duration,
		StatusCode:  r.StatusCode,
		ContentType: r.ContentType,
		Body:        r.Body,
		Error:       r.Error,
	}
}
