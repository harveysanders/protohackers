package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/maragudk/migrate"
	_ "github.com/mattn/go-sqlite3"
)

var (
	ErrDSNRequired = errors.New("DSN required")
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB represents the database connection.
// Based off Ben Johnson's WTF Dial example.
// https://github.com/benbjohnson/wtf
type DB struct {
	DB     *sql.DB
	ctx    context.Context // background context
	cancel func()          // cancel background context

	// Data source name.
	DSN string

	// Returns the current time. Defaults to time.Now().
	// Can be mocked for tests.
	Now func() time.Time
}

func NewDB(dsn string) *DB {
	db := &DB{
		DSN: dsn,
		Now: time.Now,
	}
	db.ctx, db.cancel = context.WithCancel(context.Background())
	return db
}

// Open opens the database connection.
func (db *DB) Open(drop bool) (err error) {
	// Ensure a DSN is set before attempting to open the database.
	if db.DSN == "" {
		return fmt.Errorf("dsn required")
	}

	// Make the parent directory unless using an in-memory db.
	if db.DSN != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(db.DSN), 0700); err != nil {
			return err
		}
	}

	// Connect to the database.
	if db.DB, err = sql.Open("sqlite3", db.DSN); err != nil {
		return err
	}

	// Enable WAL. SQLite performs better with the WAL  because it allows
	// multiple readers to operate while data is being written.
	if _, err := db.DB.Exec(`PRAGMA journal_mode = wal;`); err != nil {
		return fmt.Errorf("enable wal: %w", err)
	}

	// Enable many checks for better data integrity, compatibility, and
	// error checking.
	if _, err := db.DB.Exec(`PRAGMA strict = ON;`); err != nil {
		return fmt.Errorf("foreign keys pragma: %w", err)
	}

	if drop {
		if err := db.migrateDown(); err != nil {
			return fmt.Errorf("migrateDown: %w", err)
		}
	}

	if err := db.migrateUp(); err != nil {
		return fmt.Errorf("migrateUp: %w", err)
	}

	return nil
}

func (db *DB) Close() error {
	db.cancel()
	if db.DB == nil {
		return nil
	}
	return db.DB.Close()
}

func (db *DB) migrateUp() error {
	dirFS, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("fs.Sub: %w", err)
	}
	if err := migrate.Up(context.Background(), db.DB, dirFS); err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

func (db *DB) migrateDown() error {
	dirFS, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("fs.Sub: %w", err)
	}
	if err := migrate.Down(context.Background(), db.DB, dirFS); err != nil {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}
