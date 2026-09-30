package logfile

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var validName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,62}$`)

// File is the log file of one run and the logger that writes to it.
type File struct {
	file   *os.File
	logger *slog.Logger
}

// Open creates dir when it is missing and the file <name>-<time>.log in it,
// where <time> is now formatted as 20060102-150405 in the location of now.
// It fails when the file already exists. Records below level are dropped.
func Open(dir, name string, level slog.Level, now time.Time) (*File, error) {
	if dir == "" {
		return nil, errors.New("logfile: dir is required")
	}
	if !validName.MatchString(name) {
		return nil, fmt.Errorf("logfile: name %q must match %s", name, validName)
	}
	path := filepath.Join(dir, name+"-"+now.Format("20060102-150405")+".log")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("logfile: open %s: %w", path, err)
	}
	// O_EXCL: two runs started in the same second fail instead of mixing logs.
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return nil, fmt.Errorf("logfile: open %s: %w", path, err)
	}
	return &File{
		file:   file,
		logger: slog.New(slog.NewTextHandler(file, &slog.HandlerOptions{Level: level})),
	}, nil
}

// Logger returns the logger that writes to the file.
func (f *File) Logger() *slog.Logger { return f.logger }

// Path returns the path of the file.
func (f *File) Path() string { return f.file.Name() }

// Close flushes the file to disk and closes it.
func (f *File) Close() error {
	if err := errors.Join(f.file.Sync(), f.file.Close()); err != nil {
		return fmt.Errorf("logfile: close %s: %w", f.Path(), err)
	}
	return nil
}
