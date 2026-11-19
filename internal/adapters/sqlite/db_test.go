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
	if version != 18 {
		t.Fatalf("schema version = %d, want 18", version)
	}
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}
	version, err = store.SchemaVersion(ctx)
	if err != nil || version != 18 {
		t.Fatalf("schema version after repeat = %d, want 18, err = %v", version, err)
	}
	var foreignKeys int
	if err := store.DB().QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 {
		t.Fatal("foreign key enforcement is disabled")
	}
}

func TestCloudBaseSnapshotMigrationUpgradesVersion16Database(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "upgrade-v16.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.DB().ExecContext(ctx, "ALTER TABLE cloud_document_map DROP COLUMN base_data"); err != nil {
		t.Fatal(err)
	}
	if _, err = store.DB().ExecContext(ctx, "DELETE FROM schema_migrations WHERE version=17"); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, path)
	if err != nil {
		t.Fatalf("open after migration: %v", err)
	}
	defer store.Close()
	version, err := store.SchemaVersion(ctx)
	if err != nil || version != 18 {
		t.Fatalf("upgraded schema=%d err=%v", version, err)
	}
	if _, err = store.DB().ExecContext(ctx, "INSERT INTO cloud_document_map(kind,cloud_id,local_id,remote_version,updated_at) VALUES('customer','upgrade-test',1,1,'now')"); err != nil {
		t.Fatalf("v16 map rows must receive an empty merge base: %v", err)
	}
	var base string
	if err = store.DB().QueryRowContext(ctx, "SELECT base_data FROM cloud_document_map WHERE cloud_id='upgrade-test'").Scan(&base); err != nil || base != "" {
		t.Fatalf("migration base=%q err=%v", base, err)
	}
}

func TestTemplateSyncMigrationQueuesPreviouslySavedTemplates(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	result, err := store.DB().ExecContext(ctx, `INSERT INTO document_templates(name,page_size,orientation,font_family,font_size,primary_color,accent_color,created_at,updated_at)
		VALUES('قالب قدیمی','A4','portrait','IRANSansX',11,'#192631','#7058ED','now','now')`)
	if err != nil {
		t.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.DB().ExecContext(ctx, "DELETE FROM local_cloud_changes WHERE kind='template'"); err != nil {
		t.Fatal(err)
	}
	script, err := migrationFiles.ReadFile("migrations/014_template_sync_backfill.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.DB().ExecContext(ctx, string(script)); err != nil {
		t.Fatal(err)
	}
	var queued int
	if err = store.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM local_cloud_changes WHERE kind='template' AND local_id=? AND operation='upsert'", id).Scan(&queued); err != nil || queued != 1 {
		t.Fatalf("legacy template sync events=%d err=%v", queued, err)
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
