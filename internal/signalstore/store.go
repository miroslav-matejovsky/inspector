package signalstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	// Registers the database/sql driver "sqlite3".
	_ "github.com/ncruces/go-sqlite3/driver"
)

// Record is one stored signal.
type Record struct {
	Target      string        // name of the target that was read
	URL         string        // URL that was read
	ObservedAt  time.Time     // start of the read; stored as Unix nanoseconds, read back in UTC
	Duration    time.Duration // time the read took
	StatusCode  int           // HTTP status; 0 when no response was received
	ContentType string        // Content-Type header of the response; empty when none
	Body        []byte        // response body; nil when none was stored
	Error       string        // why the read failed; empty when it succeeded
}

// Stats is what the store holds for one target.
type Stats struct {
	Target string
	Count  int64  // number of stored records of the target
	Latest Record // the record of the target that was inserted last
}

// Store keeps records in one SQLite file. It is safe for concurrent use:
// all operations share one connection and run one after another.
type Store struct {
	db *sql.DB
}

// Open opens the SQLite file at path. It creates the directory and the file
// when they are missing, and creates the schema in a new file.
func Open(ctx context.Context, path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("signalstore: path is required")
	}
	if strings.ContainsAny(path, "?#") {
		return nil, fmt.Errorf("signalstore: path %q must not contain '?' or '#'", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("signalstore: open %s: %w", path, err)
	}
	// WAL lets other SQLite clients read the file while it is written; the
	// busy timeout makes a locked write wait instead of failing at once.
	dsn := "file:" + filepath.ToSlash(path) +
		"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_txlock=immediate"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("signalstore: open %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	if err := migrate(ctx, db); err != nil {
		return nil, fmt.Errorf("signalstore: open %s: %w", path, errors.Join(err, db.Close()))
	}
	return &Store{db: db}, nil
}

// Insert stores records in one transaction. An empty slice does nothing.
func (s *Store) Insert(ctx context.Context, records []Record) error {
	if len(records) == 0 {
		return nil
	}
	if err := s.insert(ctx, records); err != nil {
		return fmt.Errorf("signalstore: insert: %w", err)
	}
	return nil
}

func (s *Store) insert(ctx context.Context, records []Record) (err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		}
	}()
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO signals
		(target, url, observed_at, duration_ns, status_code, content_type, body, error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	for _, r := range records {
		if _, err := stmt.ExecContext(ctx, r.Target, r.URL, r.ObservedAt.UnixNano(), int64(r.Duration),
			r.StatusCode, r.ContentType, r.Body, r.Error); err != nil {
			return errors.Join(err, stmt.Close())
		}
	}
	if err := stmt.Close(); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteBefore deletes every record with ObservedAt before t. It returns the
// number of deleted records. Records observed exactly at t are kept.
func (s *Store) DeleteBefore(ctx context.Context, t time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, "DELETE FROM signals WHERE observed_at < ?", t.UnixNano())
	if err != nil {
		return 0, fmt.Errorf("signalstore: delete: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("signalstore: delete: %w", err)
	}
	return n, nil
}

// Stats returns one entry per target that has records, ordered by target name.
func (s *Store) Stats(ctx context.Context) ([]Stats, error) {
	stats, err := s.stats(ctx)
	if err != nil {
		return nil, fmt.Errorf("signalstore: stats: %w", err)
	}
	return stats, nil
}

func (s *Store) stats(ctx context.Context) (stats []Stats, err error) {
	// IDs only grow and DeleteBefore removes old records, so max(id) is the
	// record of a target that was inserted last.
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.target, c.n, s.url, s.observed_at, s.duration_ns, s.status_code, s.content_type, s.body, s.error
		FROM (SELECT target, count(*) AS n, max(id) AS last_id FROM signals GROUP BY target) AS c
		JOIN signals AS s ON s.id = c.last_id
		ORDER BY s.target`)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, rows.Close()) }()

	stats = []Stats{}
	for rows.Next() {
		var st Stats
		var observedAt, duration int64
		r := &st.Latest
		if err := rows.Scan(&st.Target, &st.Count, &r.URL, &observedAt, &duration,
			&r.StatusCode, &r.ContentType, &r.Body, &r.Error); err != nil {
			return nil, err
		}
		r.Target = st.Target
		r.ObservedAt = time.Unix(0, observedAt).UTC()
		r.Duration = time.Duration(duration)
		stats = append(stats, st)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return stats, nil
}

// Close closes the database.
func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("signalstore: close: %w", err)
	}
	return nil
}
