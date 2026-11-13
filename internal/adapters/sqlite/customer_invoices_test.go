package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/hesora-ir/hesora-free/internal/domain"
)

func TestCustomerInvoiceHistoryUsesCustomerID(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	first, err := store.CreateCustomer(ctx, domain.Customer{Name: "مشتری هم‌نام"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.CreateCustomer(ctx, domain.Customer{Name: "مشتری هم‌نام"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, row := range []struct {
		customer                       int64
		number, kind, status, currency string
		amount                         int64
	}{
		{first, "FIRST-ISSUED", "invoice", "issued", "IRT", 1200},
		{first, "FIRST-PROFORMA", "proforma", "draft", "IRR", 9500},
		{first, "FIRST-VOID", "invoice", "void", "IRT", 100},
		{second, "SECOND-ISSUED", "invoice", "issued", "IRT", 700},
	} {
		_, err := store.db.ExecContext(ctx, `INSERT INTO invoices
            (number,kind,status,currency,customer_id,issue_date,subtotal_minor,total_minor,additional_charge_minor,created_at,updated_at)
            VALUES(?,?,?,?,?,'2026-11-13',?,?,50,?,?)`, row.number, row.kind, row.status, row.currency, row.customer, row.amount, row.amount, now, now)
		if err != nil {
			t.Fatal(err)
		}
	}
	rows, err := store.CustomerInvoices(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("want issued invoice and proforma, got %+v", rows)
	}
	seen := map[string]domain.CustomerInvoice{}
	for _, row := range rows {
		seen[row.Number] = row
	}
	invoice, ok := seen["FIRST-ISSUED"]
	if !ok || invoice.Total != 1250 || invoice.Balance != 1250 || invoice.Currency != domain.CurrencyToman {
		t.Fatalf("incorrect invoice totals or currency: %+v", rows)
	}
	if _, ok := seen["FIRST-PROFORMA"]; !ok {
		t.Fatal("missing proforma history")
	}
	other, err := store.CustomerInvoices(ctx, second)
	if err != nil || len(other) != 1 || other[0].Number != "SECOND-ISSUED" {
		t.Fatalf("customer histories mixed: %+v, %v", other, err)
	}
	if _, err := store.CustomerInvoices(ctx, 0); err == nil {
		t.Fatal("invalid customer ID must fail")
	}
}
