package logfile_test

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/internal/logfile"
)

var fixed = time.Date(2026, 9, 30, 3, 52, 32, 0, time.UTC)

func content(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}

func TestOpenCreatesFile(t *testing.T) {
	dir := t.TempDir()

	f, err := logfile.Open(filepath.Join(dir, "logs"), "app", slog.LevelInfo, fixed)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, f.Close()) })

	require.Equal(t, filepath.Join(dir, "logs", "app-20260930-035232.log"), f.Path())
	require.FileExists(t, f.Path())
}

func TestLoggerWritesToFile(t *testing.T) {
	f, err := logfile.Open(t.TempDir(), "app", slog.LevelInfo, fixed)
	require.NoError(t, err)

	f.Logger().Info("hello", "k", "v")
	require.NoError(t, f.Close())

	require.Contains(t, content(t, f.Path()), "level=INFO msg=hello k=v")
}

func TestLevelFiltersRecords(t *testing.T) {
	f, err := logfile.Open(t.TempDir(), "app", slog.LevelWarn, fixed)
	require.NoError(t, err)

	f.Logger().Info("quiet")
	f.Logger().Warn("loud")
	require.NoError(t, f.Close())

	got := content(t, f.Path())
	require.Contains(t, got, "msg=loud")
	require.NotContains(t, got, "msg=quiet")
}

func TestOpenKeepsExistingFile(t *testing.T) {
	dir := t.TempDir()
	f, err := logfile.Open(dir, "app", slog.LevelInfo, fixed)
	require.NoError(t, err)
	f.Logger().Info("first")
	require.NoError(t, f.Close())

	_, err = logfile.Open(dir, "app", slog.LevelInfo, fixed)

	require.ErrorIs(t, err, fs.ErrExist)
	require.Contains(t, content(t, f.Path()), "msg=first")
}

func TestOpenRejectsArguments(t *testing.T) {
	dir := t.TempDir()
	tests := map[string]struct{ dir, name string }{
		"empty dir":  {"", "app"},
		"empty name": {dir, ""},
		"upper case": {dir, "App"},
		"separator":  {dir, "a/b"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := logfile.Open(tc.dir, tc.name, slog.LevelInfo, fixed)

			require.Error(t, err)
		})
	}
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, entries)
}
