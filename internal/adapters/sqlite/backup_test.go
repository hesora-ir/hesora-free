package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hesora-ir/hesora-free/internal/domain"
)

func TestBackupCapturesCommittedWALAndNeverOverwrites(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	if _, err := s.CreateCustomer(ctx, domain.Customer{Name: "بکاپ ابری"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "backup.db")
	if err := s.BackupTo(ctx, path); err != nil {
		t.Fatal(err)
	}
	copy, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer copy.Close()
	var name string
	if err := copy.DB().QueryRowContext(ctx, "SELECT name FROM customers").Scan(&name); err != nil || name != "بکاپ ابری" {
		t.Fatalf("name=%q err=%v", name, err)
	}
	if err := s.BackupTo(ctx, path); err == nil {
		t.Fatal("overwrote an existing backup")
	}
	if err := s.BackupTo(ctx, s.Path()); err == nil {
		t.Fatal("overwrote live database")
	}
	existing := filepath.Join(t.TempDir(), "keep.db")
	if err := os.WriteFile(existing, []byte("keep me"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.BackupTo(ctx, existing); err == nil {
		t.Fatal("overwrote user file")
	}
	data, _ := os.ReadFile(existing)
	if string(data) != "keep me" {
		t.Fatal("destination changed")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	missing := filepath.Join(t.TempDir(), "cancelled.db")
	if err := s.BackupTo(cancelled, missing); err == nil {
		t.Fatal("ignored cancellation")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("partial backup left behind")
	}
}
