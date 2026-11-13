package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// BackupTo takes a consistent snapshot, including committed WAL data. It never
// overwrites an existing file, even if it is the running application's database.
func (s *Store) BackupTo(ctx context.Context, destination string) error {
	if destination == "" {
		return errors.New("مسیر بکاپ خالی است")
	}
	abs, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(abs), ".hesora-backup-*.db")
	if err != nil {
		return err
	}
	path := tmp.Name()
	defer os.Remove(path)
	if err = tmp.Close(); err != nil {
		return err
	}
	if _, err = s.db.ExecContext(ctx, "VACUUM INTO ?", path); err != nil {
		return fmt.Errorf("ساخت بکاپ: %w", err)
	}
	check, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	var integrity string
	err = check.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&integrity)
	closeErr := check.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if integrity != "ok" {
		return errors.New("کنترل سلامت بکاپ ناموفق بود")
	}
	source, err := os.Open(path)
	if err != nil {
		return err
	}
	defer source.Close()
	output, err := os.OpenFile(abs, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("فایل مقصد باید جدید باشد: %w", err)
	}
	complete := false
	defer func() {
		output.Close()
		if !complete {
			os.Remove(abs)
		}
	}()
	if _, err = io.Copy(output, source); err != nil {
		return err
	}
	if err = output.Sync(); err != nil {
		return err
	}
	if err = output.Close(); err != nil {
		return err
	}
	complete = true
	return nil
}
