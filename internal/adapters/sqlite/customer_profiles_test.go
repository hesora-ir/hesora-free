package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/hesora-ir/hesora-free/internal/domain"
)

func TestCustomerProfilePersistsMultiplePhonesAndAddresses(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "customers.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	id, err := store.CreateCustomer(ctx, domain.Customer{
		Type: domain.CustomerTypeIndividual, FirstName: "آزمایشی", LastName: "مشتری",
		Phones:    []string{"phone-a", "phone-b"},
		Addresses: []string{"address-a", "address-b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.CustomerByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != domain.CustomerTypeIndividual || got.Name != "آزمایشی مشتری" {
		t.Fatalf("unexpected customer identity: %+v", got)
	}
	if len(got.Phones) != 2 || got.Phones[1] != "phone-b" {
		t.Fatalf("unexpected phones: %#v", got.Phones)
	}
	if len(got.Addresses) != 2 || got.Addresses[1] != "address-b" {
		t.Fatalf("unexpected addresses: %#v", got.Addresses)
	}

	got.Phones = []string{"phone-c"}
	got.Addresses = []string{"address-c"}
	if err := store.UpdateCustomer(ctx, got); err != nil {
		t.Fatal(err)
	}
	got, err = store.CustomerByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Phones) != 1 || got.Phones[0] != "phone-c" {
		t.Fatalf("unexpected updated phones: %#v", got.Phones)
	}
	if len(got.Addresses) != 1 || got.Addresses[0] != "address-c" {
		t.Fatalf("unexpected updated addresses: %#v", got.Addresses)
	}
}

func TestCustomerCardsAggregateIssuedInvoicesAndDebtByCurrency(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "customer-stats.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	customerID, err := store.CreateCustomer(ctx, domain.Customer{Type: domain.CustomerTypeCompany, LegalName: "شرکت آزمایشی", TaxID: "tax-test"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, invoice := range []struct {
		number, kind, status, currency string
		total                          int64
	}{
		{"TEST-INV-1", "invoice", "issued", "IRT", 1500},
		{"TEST-PRO-1", "proforma", "draft", "IRT", 9000},
		{"TEST-INV-2", "invoice", "issued", "IRR", 700},
	} {
		_, err := store.db.ExecContext(ctx, "INSERT INTO invoices(number,kind,status,currency,customer_id,customer_snapshot,issuer_snapshot,issue_date,subtotal_minor,discount_minor,tax_minor,total_minor,created_at,updated_at) VALUES(?,?,?, ?,?,'{}','{}','2026-10-25',?,0,0,?,?,?)",
			invoice.number, invoice.kind, invoice.status, invoice.currency, customerID, invoice.total, invoice.total, now, now)
		if err != nil {
			t.Fatal(err)
		}
	}
	customers, err := store.ListCustomers(ctx, "شرکت آزمایشی")
	if err != nil {
		t.Fatal(err)
	}
	if len(customers) != 1 {
		t.Fatalf("got %d customers, want 1", len(customers))
	}
	got := customers[0]
	if got.IssuedInvoiceCount != 2 || got.OpenInvoiceCount != 2 || got.ProformaCount != 1 {
		t.Fatalf("unexpected invoice counts: %+v", got)
	}
	if got.IssuedTotalIRT != 1500 || got.IssuedTotalIRR != 700 || got.DebtIRT != 1500 || got.DebtIRR != 700 {
		t.Fatalf("unexpected invoice totals/debt: %+v", got)
	}
}
