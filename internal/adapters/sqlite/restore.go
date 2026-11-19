package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	modernsqlite "modernc.org/sqlite"
)

type sqliteBackupConn interface {
	NewBackup(string) (*modernsqlite.Backup, error)
	NewRestore(string) (*modernsqlite.Backup, error)
}

// RestoreFrom restores a compatible SQLite snapshot into the open workspace.
// A consistent pre-restore snapshot is retained beside the live database.
func (s *Store) RestoreFrom(ctx context.Context, sourcePath string) (string, error) {
	if strings.TrimSpace(sourcePath) == "" {
		return "", errors.New("مسیر فایل بازیابی خالی است")
	}
	source, err := filepath.Abs(sourcePath)
	if err != nil {
		return "", err
	}
	current, err := filepath.Abs(s.path)
	if err != nil {
		return "", err
	}
	if samePath(source, current) {
		return "", errors.New("فایل بازیابی نمی‌تواند پایگاه دادهٔ در حال استفاده باشد")
	}
	currentVersion, err := s.SchemaVersion(ctx)
	if err != nil {
		return "", err
	}
	if err = validateRestoreSource(ctx, source, currentVersion); err != nil {
		return "", err
	}
	safety, err := os.CreateTemp(filepath.Dir(current), ".hesora-pre-restore-*.db")
	if err != nil {
		return "", err
	}
	safetyPath := safety.Name()
	if err = safety.Close(); err != nil {
		os.Remove(safetyPath)
		return "", err
	}
	if err = os.Remove(safetyPath); err != nil {
		return "", err
	}
	if err = copySQLite(ctx, s.db, safetyPath, false); err != nil {
		os.Remove(safetyPath)
		return "", fmt.Errorf("ساخت نسخهٔ برگشت پیش از بازیابی ناموفق بود: %w", err)
	}
	if err = copySQLite(ctx, s.db, source, true); err != nil {
		_ = copySQLite(context.Background(), s.db, safetyPath, true)
		return "", fmt.Errorf("بازیابی ناموفق بود؛ نسخهٔ قبلی برگردانده شد: %w", err)
	}
	if err = s.Migrate(ctx); err == nil {
		err = checkIntegrity(ctx, s.db)
	}
	if err != nil {
		rollbackErr := copySQLite(context.Background(), s.db, safetyPath, true)
		if rollbackErr != nil {
			return safetyPath, fmt.Errorf("بازیابی نیازمند ترمیم بود و بازگردانی خودکار هم موفق نشد؛ نسخهٔ قبل در %s است: %v (ترمیم: %w)", safetyPath, err, rollbackErr)
		}
		return "", fmt.Errorf("فایل بازیابی با این نسخه سازگار نبود؛ پایگاه قبلی بازگردانده شد: %w", err)
	}
	return safetyPath, nil
}

func validateRestoreSource(ctx context.Context, path string, currentVersion int) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() < 100 {
		return errors.New("فایل انتخاب‌شده بکاپ SQLite معتبر نیست")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err = db.ExecContext(ctx, "PRAGMA query_only=ON"); err != nil {
		return err
	}
	if err = checkIntegrity(ctx, db); err != nil {
		return err
	}
	var hasMigrations int
	if err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'").Scan(&hasMigrations); err != nil {
		return err
	}
	if hasMigrations != 1 {
		return errors.New("نسخهٔ فایل بکاپ قابل شناسایی نیست")
	}
	var version int
	if err = db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version),0) FROM schema_migrations").Scan(&version); err != nil {
		return err
	}
	if version < 1 {
		return errors.New("بکاپ ساختار دادهٔ هسورا را ندارد")
	}
	if version > currentVersion {
		return fmt.Errorf("نسخهٔ بکاپ (%d) از نسخهٔ این برنامه (%d) جدیدتر است", version, currentVersion)
	}
	for _, table := range []string{"customers", "invoices", "issuer_profiles"} {
		var exists int
		if err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&exists); err != nil {
			return err
		}
		if exists != 1 {
			return fmt.Errorf("جدول اصلی %s در بکاپ وجود ندارد", table)
		}
	}
	return nil
}

func checkIntegrity(ctx context.Context, db *sql.DB) error {
	var state string
	if err := db.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&state); err != nil {
		return err
	}
	if state != "ok" {
		return fmt.Errorf("آزمون سلامت SQLite ناموفق بود: %s", state)
	}
	return nil
}

func copySQLite(ctx context.Context, db *sql.DB, path string, restore bool) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.Raw(func(raw any) error {
		backupConn, ok := raw.(sqliteBackupConn)
		if !ok {
			return errors.New("درایور SQLite از بازیابی آنلاین پشتیبانی نمی‌کند")
		}
		var b *modernsqlite.Backup
		var e error
		if restore {
			b, e = backupConn.NewRestore(path)
		} else {
			b, e = backupConn.NewBackup(path)
		}
		if e != nil {
			return e
		}
		finished := false
		defer func() {
			if !finished {
				_ = b.Finish()
			}
		}()
		for {
			more, stepErr := b.Step(256)
			if stepErr != nil {
				return stepErr
			}
			if !more {
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(10 * time.Millisecond):
			}
		}
		e = b.Finish()
		finished = true
		return e
	})
}

func samePath(a, b string) bool {
	if os.PathSeparator == '\\' {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}
