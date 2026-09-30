package signalstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// schemaVersion is stored in PRAGMA user_version.
const schemaVersion = 1

const schema = `
CREATE TABLE signals (
    id           INTEGER PRIMARY KEY,
    target       TEXT    NOT NULL,
    url          TEXT    NOT NULL,
    observed_at  INTEGER NOT NULL, -- Unix nanoseconds
    duration_ns  INTEGER NOT NULL,
    status_code  INTEGER NOT NULL,
    content_type TEXT    NOT NULL,
    body         BLOB,             -- NULL when no body was stored
    error        TEXT    NOT NULL
) STRICT;
CREATE INDEX signals_by_target ON signals (target, id);
CREATE INDEX signals_by_time ON signals (observed_at);
`

// migrate creates the schema in a new database and rejects any schema
// version other than schemaVersion. There are no migrations between versions.
func migrate(ctx context.Context, db *sql.DB) (err error) {
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	switch version {
	case schemaVersion:
		return nil
	case 0:
	default:
		return fmt.Errorf("schema version %d, want %d: delete the file to start empty", version, schemaVersion)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		}
	}()
	if _, err := tx.ExecContext(ctx, schema); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
		return err
	}
	return tx.Commit()
}
