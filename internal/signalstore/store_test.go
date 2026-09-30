package signalstore_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/internal/signalstore"
)

var t0 = time.Date(2026, 9, 30, 1, 2, 3, 0, time.UTC)

// open opens a store at path and closes it when the test ends.
func open(t *testing.T, path string) *signalstore.Store {
	t.Helper()
	s, err := signalstore.Open(context.Background(), path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })
	return s
}

func record(target string, at time.Time, body string) signalstore.Record {
	return signalstore.Record{
		Target:      target,
		URL:         "http://example.test/" + target,
		ObservedAt:  at,
		Duration:    1500 * time.Microsecond,
		StatusCode:  200,
		ContentType: "text/plain",
		Body:        []byte(body),
	}
}

func TestOpenCreatesDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "s.db")

	s := open(t, path)

	require.FileExists(t, path)
	stats, err := s.Stats(context.Background())
	require.NoError(t, err)
	require.Empty(t, stats)
}

func TestOpenReopensDatabase(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "s.db")
	s, err := signalstore.Open(ctx, path)
	require.NoError(t, err)
	require.NoError(t, s.Insert(ctx, []signalstore.Record{record("alpha", t0, "1"), record("alpha", t0, "2")}))
	require.NoError(t, s.Close())

	stats, err := open(t, path).Stats(ctx)

	require.NoError(t, err)
	require.Len(t, stats, 1)
	require.Equal(t, int64(2), stats[0].Count)
}

func TestOpenRejectsPath(t *testing.T) {
	dir := t.TempDir()
	for _, path := range []string{"", filepath.Join(dir, "a?b.db"), filepath.Join(dir, "a#b.db")} {
		t.Run(path, func(t *testing.T) {
			_, err := signalstore.Open(context.Background(), path)

			require.ErrorContains(t, err, "signalstore")
		})
	}
}

func TestOpenRejectsForeignFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.db")
	require.NoError(t, os.WriteFile(path, []byte("not a database"), 0o644))

	_, err := signalstore.Open(context.Background(), path)

	require.Error(t, err)
}

func TestOpenRejectsUnknownSchemaVersion(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "s.db")
	s, err := signalstore.Open(ctx, path)
	require.NoError(t, err)
	require.NoError(t, s.Close())
	db, err := sql.Open("sqlite3", "file:"+filepath.ToSlash(path))
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "PRAGMA user_version = 2")
	require.NoError(t, err)
	require.NoError(t, db.Close())

	_, err = signalstore.Open(ctx, path)

	require.ErrorContains(t, err, "schema version 2")
}

func TestInsertAndStats(t *testing.T) {
	ctx := context.Background()
	s := open(t, filepath.Join(t.TempDir(), "s.db"))
	alpha2 := signalstore.Record{
		Target: "alpha", URL: "http://example.test/a", ObservedAt: t0.Add(time.Second),
		Duration: 2 * time.Millisecond, StatusCode: 503, ContentType: "application/json",
		Body: []byte(`{"s":"down"}`), Error: "",
	}
	beta1 := record("beta", t0, "beta body")

	require.NoError(t, s.Insert(ctx, []signalstore.Record{record("alpha", t0, "alpha body"), beta1, alpha2}))

	stats, err := s.Stats(ctx)
	require.NoError(t, err)
	require.Equal(t, []signalstore.Stats{
		{Target: "alpha", Count: 2, Latest: alpha2},
		{Target: "beta", Count: 1, Latest: beta1},
	}, stats)
}

func TestInsertKeepsFailedRead(t *testing.T) {
	ctx := context.Background()
	s := open(t, filepath.Join(t.TempDir(), "s.db"))
	failed := signalstore.Record{Target: "alpha", URL: "http://example.test/a", ObservedAt: t0, Error: "boom"}

	require.NoError(t, s.Insert(ctx, []signalstore.Record{failed}))

	stats, err := s.Stats(ctx)
	require.NoError(t, err)
	require.Len(t, stats, 1)
	require.Nil(t, stats[0].Latest.Body)
	require.Equal(t, 0, stats[0].Latest.StatusCode)
	require.Equal(t, "boom", stats[0].Latest.Error)
}

func TestInsertNothing(t *testing.T) {
	ctx := context.Background()
	s := open(t, filepath.Join(t.TempDir(), "s.db"))

	require.NoError(t, s.Insert(ctx, nil))

	stats, err := s.Stats(ctx)
	require.NoError(t, err)
	require.Empty(t, stats)
}

func TestDeleteBefore(t *testing.T) {
	ctx := context.Background()
	s := open(t, filepath.Join(t.TempDir(), "s.db"))
	latest := record("alpha", t0.Add(2*time.Second), "3")
	require.NoError(t, s.Insert(ctx, []signalstore.Record{
		record("alpha", t0, "1"), record("alpha", t0.Add(time.Second), "2"), latest,
	}))

	deleted, err := s.DeleteBefore(ctx, t0.Add(time.Second))

	require.NoError(t, err)
	require.Equal(t, int64(1), deleted)
	stats, err := s.Stats(ctx)
	require.NoError(t, err)
	require.Equal(t, []signalstore.Stats{{Target: "alpha", Count: 2, Latest: latest}}, stats)
}

func TestStoreFailsAfterClose(t *testing.T) {
	ctx := context.Background()
	s, err := signalstore.Open(ctx, filepath.Join(t.TempDir(), "s.db"))
	require.NoError(t, err)
	require.NoError(t, s.Close())

	err = s.Insert(ctx, []signalstore.Record{record("alpha", t0, "1")})

	require.Error(t, err)
}
