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
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"

	"github.com/hesora-ir/hesora-free/internal/platform/datadir"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Store struct {
	db         *sql.DB
	path       string
	proEnabled atomic.Bool
}

func (s *Store) SetProEnabled(enabled bool) { s.proEnabled.Store(enabled) }
func (s *Store) ProEnabled() bool           { return s.proEnabled.Load() }

func OpenDefault(ctx context.Context) (*Store, error) {
	dir, err := datadir.Ensure()
	if err != nil {
		return nil, fmt.Errorf("آماده‌سازی پوشه داده برنامه: %w", err)
	}
	return Open(ctx, filepath.Join(dir, "invoice-maker.db"))
}

func Open(ctx context.Context, path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("مسیر پایگاه داده خالی است")
	}
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return nil, fmt.Errorf("ایجاد پوشه پایگاه داده: %w", err)
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("بازکردن SQLite: %w", err)
	}
	// A single connection keeps connection-local PRAGMAs deterministic for this
	// desktop application's initial storage layer.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	fail := func(err error) (*Store, error) {
		_ = db.Close()
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		return fail(fmt.Errorf("اتصال به SQLite: %w", err))
	}
	for _, pragma := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
	} {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			return fail(fmt.Errorf("تنظیم SQLite (%s): %w", pragma, err))
		}
	}
	store := &Store{db: db, path: path}
	if err := store.Migrate(ctx); err != nil {
		return fail(err)
	}
	return store, nil
}

func (s *Store) DB() *sql.DB  { return s.db }
func (s *Store) Path() string { return s.path }
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TEXT NOT NULL
		)`); err != nil {
		return fmt.Errorf("ایجاد جدول نسخه مهاجرت‌ها: %w", err)
	}

	entries, err := fs.Glob(migrationFiles, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("فهرست migrationها: %w", err)
	}
	sort.Strings(entries)
	for _, name := range entries {
		base := filepath.Base(name)
		parts := strings.SplitN(base, "_", 2)
		if len(parts) != 2 {
			return fmt.Errorf("نام migration نامعتبر است: %s", base)
		}
		version, err := strconv.Atoi(parts[0])
		if err != nil || version < 1 {
			return fmt.Errorf("نسخه migration نامعتبر است: %s", base)
		}
		var exists int
		err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&exists)
		if err != nil {
			return fmt.Errorf("بررسی migration %s: %w", base, err)
		}
		if exists != 0 {
			continue
		}
		content, err := migrationFiles.ReadFile(name)
		if err != nil {
			return fmt.Errorf("خواندن migration %s: %w", base, err)
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("شروع تراکنش migration %s: %w", base, err)
		}
		for _, statement := range strings.Split(string(content), ";") {
			statement = strings.TrimSpace(statement)
			if statement == "" {
				continue
			}
			if _, err = tx.ExecContext(ctx, statement); err != nil {
				break
			}
		}
		if err == nil {
			_, err = tx.ExecContext(ctx,
				"INSERT INTO schema_migrations(version, name, applied_at) VALUES(?, ?, ?)",
				version, base, time.Now().UTC().Format(time.RFC3339Nano))
		}
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("اجرای migration %s: %w", base, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("ثبت migration %s: %w", base, err)
		}
	}
	return nil
}

func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	var version int
	err := s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	return version, err
}
