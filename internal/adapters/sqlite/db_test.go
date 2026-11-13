package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/hesora-ir/hesora-free/internal/domain"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestMigrationsAreVersionedAndIdempotent(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	version, err := store.SchemaVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if version != 10 {
		t.Fatalf("schema version = %d, want 10", version)
	}
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}
	version, err = store.SchemaVersion(ctx)
	if err != nil || version != 10 {
		t.Fatalf("schema version after repeat = %d, err = %v", version, err)
	}
	var foreignKeys int
	if err := store.DB().QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 {
		t.Fatal("foreign key enforcement is disabled")
	}
}

func TestCustomerRoundTrip(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	id, err := store.CreateCustomer(ctx, domain.Customer{Name: "مشتری آزمایشی", Phone: "021"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.CustomerByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "مشتری آزمایشی" || got.Phone != "021" || got.ID != id {
		t.Fatalf("unexpected customer: %+v", got)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatal("expected timestamps to be populated")
	}
}

func TestCustomerValidationAndNotFound(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	if _, err := store.CreateCustomer(ctx, domain.Customer{}); err == nil {
		t.Fatal("expected empty customer to fail validation")
	}
	if _, err := store.CustomerByID(ctx, 999); err == nil {
		t.Fatal("expected missing customer error")
	}
}

func TestInvoiceNumberUniqueAndForeignKeys(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	insert := `INSERT INTO invoices(number, kind, status, currency, customer_id, issue_date, subtotal_minor, discount_minor, tax_minor, total_minor, created_at, updated_at)
		VALUES(?, 'invoice', 'draft', 'IRT', ?, ?, 100, 0, 0, 100, ?, ?)`
	if _, err := store.DB().ExecContext(ctx, insert, "INV-1", 987654, now, now, now); err == nil {
		t.Fatal("expected foreign key violation")
	}
	if _, err := store.DB().ExecContext(ctx, insert, "INV-1", nil, now, now, now); err != nil {
		t.Fatalf("insert valid invoice: %v", err)
	}
	if _, err := store.DB().ExecContext(ctx, insert, "INV-1", nil, now, now, now); err == nil {
		t.Fatal("expected duplicate invoice number to fail")
	}
}
