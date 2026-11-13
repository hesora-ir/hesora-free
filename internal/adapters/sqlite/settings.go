package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/hesora-ir/hesora-free/internal/domain"
)

func (s *Store) SetSetting(ctx context.Context, setting domain.Setting) error {
	if err := setting.Validate(); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO settings(key, value, updated_at) VALUES(?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		setting.Key, setting.Value, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("ذخیره تنظیمات: %w", err)
	}
	return nil
}

func (s *Store) GetSetting(ctx context.Context, key string) (domain.Setting, error) {
	var setting domain.Setting
	var updatedAt string
	err := s.db.QueryRowContext(ctx, "SELECT key, value, updated_at FROM settings WHERE key=?", key).
		Scan(&setting.Key, &setting.Value, &updatedAt)
	if err != nil {
		return domain.Setting{}, fmt.Errorf("خواندن تنظیمات: %w", err)
	}
	setting.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	return setting, err
}
