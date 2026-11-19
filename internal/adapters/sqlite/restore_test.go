package sqlite

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hesora-ir/hesora-free/internal/domain"
)

func TestRestoreFromValidSnapshotKeepsPreRestoreSnapshot(t *testing.T) {
	ctx := context.Background()
	target := openTestStore(t)
	if _, err := target.SaveExpense(ctx, domain.Expense{Amount: 10, Currency: domain.CurrencyToman, Date: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC), Category: "Old data"}); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(t.TempDir(), "source.db")
	source, err := Open(ctx, sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = source.SaveExpense(ctx, domain.Expense{Amount: 25, Currency: domain.CurrencyRial, Date: time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC), Category: "Restored data"}); err != nil {
		t.Fatal(err)
	}
	if err = source.Close(); err != nil {
		t.Fatal(err)
	}
	safety, err := target.RestoreFrom(ctx, sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(safety); err != nil {
		t.Fatalf("safety snapshot missing at %q: %v", safety, err)
	}
	rows, err := target.ListExpenses(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Category != "Restored data" || rows[0].Amount != 25 {
		t.Fatalf("restored rows=%+v", rows)
	}
	old, err := sql.Open("sqlite", safety)
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	var category string
	if err = old.QueryRowContext(ctx, "SELECT category FROM expenses LIMIT 1").Scan(&category); err != nil || category != "Old data" {
		t.Fatalf("saved rollback category=%q err=%v", category, err)
	}
}

func TestRestoreRejectsInvalidBackupWithoutChangingWorkspace(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	if _, err := store.SaveExpense(ctx, domain.Expense{Amount: 10, Currency: domain.CurrencyToman, Date: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC), Category: "Keep me"}); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(t.TempDir(), "bad.db")
	if err := os.WriteFile(bad, []byte("not a sqlite file"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RestoreFrom(ctx, bad); err == nil {
		t.Fatal("invalid backup was accepted")
	}
	rows, err := store.ListExpenses(ctx)
	if err != nil || len(rows) != 1 || rows[0].Category != "Keep me" {
		t.Fatalf("workspace changed after invalid restore: rows=%+v err=%v", rows, err)
	}
}

func TestRestoreRollsBackWhenSnapshotCannotPassMigrations(t *testing.T) {
	ctx := context.Background()
	target := openTestStore(t)
	if _, err := target.SaveExpense(ctx, domain.Expense{Amount: 10, Currency: domain.CurrencyToman, Date: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC), Category: "Keep current"}); err != nil {
		t.Fatal(err)
	}
	brokenPath := filepath.Join(t.TempDir(), "broken-v18.db")
	broken, err := Open(ctx, brokenPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = broken.Close(); err != nil {
		t.Fatal(err)
	}
	corrupt, err := sql.Open("sqlite", brokenPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = corrupt.ExecContext(ctx, "DROP TABLE expenses"); err != nil {
		t.Fatal(err)
	}
	if err = corrupt.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = target.RestoreFrom(ctx, brokenPath); err == nil {
		t.Fatal("snapshot missing a migration-created table was accepted")
	}
	rows, err := target.ListExpenses(ctx)
	if err != nil || len(rows) != 1 || rows[0].Category != "Keep current" {
		t.Fatalf("pre-restore data was not recovered: rows=%+v err=%v", rows, err)
	}
}
