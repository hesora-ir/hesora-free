package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/hesora-ir/hesora-free/internal/domain"
)

func testInvoiceDraft() domain.InvoiceDocument {
	return domain.InvoiceDocument{
		Kind: "invoice", Currency: domain.CurrencyToman,
		CustomerSnapshot: "{}", IssuerSnapshot: "{}",
		IssueDate:       time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		Items:           []domain.InvoiceItem{{Description: "خدمت آزمایشی", Unit: "ساعت", Quantity: 1, UnitPrice: domain.Money(1000)}},
		InvoiceDiscount: domain.Discount{Mode: domain.DiscountFixed},
		Numbering:       domain.NumberingConfig{Series: "sales", Prefix: "INV-", StartAt: 7, Width: 4},
	}
}

func TestCreateInvoiceDraftAndIssue(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "invoice.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	doc := testInvoiceDraft()
	id, number, err := store.CreateInvoiceDraft(ctx, doc)
	if err != nil {
		t.Fatal(err)
	}
	if number != "INV-0007" {
		t.Fatalf("number = %q", number)
	}
	var unit string
	if err := store.DB().QueryRowContext(ctx, "SELECT unit FROM invoice_items WHERE invoice_id=? ORDER BY position LIMIT 1", id).Scan(&unit); err != nil {
		t.Fatal(err)
	}
	if unit != "ساعت" {
		t.Fatalf("item unit = %q", unit)
	}
	id2, number2, err := store.CreateInvoiceDraft(ctx, doc)
	if err != nil {
		t.Fatal(err)
	}
	if id2 == id || number2 != "INV-0008" {
		t.Fatalf("second invoice id=%d number=%q", id2, number2)
	}
	if err := store.IssueInvoice(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := store.IssueInvoice(ctx, id); err == nil {
		t.Fatal("expected issuing non-draft invoice to fail")
	}
	balance, err := store.InvoiceBalance(ctx, id)
	if err != nil || balance != domain.Money(1000) {
		t.Fatalf("balance=%v err=%v", balance, err)
	}
}

func TestCreateInvoiceDraftRollsBackNumberingOnFailure(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "rollback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	doc := testInvoiceDraft()
	doc.CustomerID = 999999 // foreign key failure must roll back sequence and invoice
	if _, _, err := store.CreateInvoiceDraft(ctx, doc); err == nil {
		t.Fatal("expected foreign key failure")
	}
	doc.CustomerID = 0
	_, number, err := store.CreateInvoiceDraft(ctx, doc)
	if err != nil {
		t.Fatal(err)
	}
	if number != "INV-0007" {
		t.Fatalf("number after rollback = %q", number)
	}
}

func TestSuggestedInvoiceItems(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "suggestions.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	doc := testInvoiceDraft()
	doc.Items[0] = domain.InvoiceItem{Description: "کابل cat 6 لگراند", Unit: "متر", Quantity: 1, UnitPrice: 1000}
	id, _, err := store.CreateInvoiceDraft(ctx, doc)
	if err != nil {
		t.Fatal(err)
	}
	items, err := store.SuggestedInvoiceItems(ctx, "کابل", domain.CurrencyToman)
	if err != nil || len(items) != 0 {
		t.Fatalf("draft suggested: %v, %v", items, err)
	}
	if err = store.IssueInvoice(ctx, id); err != nil {
		t.Fatal(err)
	}
	doc.Items[0].UnitPrice = 2000
	id, _, err = store.CreateInvoiceDraft(ctx, doc)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.IssueInvoice(ctx, id); err != nil {
		t.Fatal(err)
	}
	items, err = store.SuggestedInvoiceItems(ctx, "کابل", domain.CurrencyToman)
	if err != nil || len(items) != 1 || items[0].UnitPrice != 2000 || items[0].Unit != "متر" {
		t.Fatalf("suggestions: %v, %v", items, err)
	}
	items, err = store.SuggestedInvoiceItems(ctx, "کابل", domain.CurrencyRial)
	if err != nil || len(items) != 0 {
		t.Fatalf("wrong currency: %v, %v", items, err)
	}
}
