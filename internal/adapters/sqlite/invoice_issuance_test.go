package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/hesora-ir/hesora-free/internal/domain"
)

func TestProformaConversionIsAtomicAndCreatesInvoiceDebtRecord(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "proforma.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	doc := testInvoiceDraft()
	doc.Kind = "proforma"
	doc.BankAccountsSnapshot = "[\"bank-a\"]"
	id, number, err := store.CreateInvoiceDraft(ctx, doc)
	if err != nil {
		t.Fatal(err)
	}
	if number == "" {
		t.Fatal("expected generated proforma number")
	}

	got, err := store.GetInvoiceDocument(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != "proforma" || got.BankAccountsSnapshot != doc.BankAccountsSnapshot || len(got.Items) != 1 {
		t.Fatalf("unexpected proforma round trip: %+v", got)
	}
	if err := store.ConvertProforma(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := store.ConvertProforma(ctx, id); err == nil {
		t.Fatal("expected duplicate conversion to fail")
	}

	list, err := store.ListInvoiceDocuments(ctx, number)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Kind != "invoice" || list[0].Status != string(domain.InvoiceIssued) || list[0].Balance != 1000 {
		t.Fatalf("unexpected converted invoice list: %+v", list)
	}
}

func TestUpdateInvoiceDraftPreservesNumberAndReplacesItemsTransactionally(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "edit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	doc := testInvoiceDraft()
	id, number, err := store.CreateInvoiceDraft(ctx, doc)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := store.GetInvoiceDocument(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	loaded.Items = []domain.InvoiceItem{{Description: "خدمت ویرایش‌شده", Unit: "عدد", Quantity: 2, UnitPrice: domain.Money(1500)}}
	if err := store.UpdateInvoiceDraft(ctx, loaded); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetInvoiceDocument(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Number != number || len(got.Items) != 1 || got.Items[0].Description != "خدمت ویرایش‌شده" || got.Items[0].Quantity != 2 {
		t.Fatalf("unexpected edited document: %+v", got)
	}
	if err := store.IssueInvoice(ctx, id); err != nil {
		t.Fatal(err)
	}
	// Unsettled issued invoices can be edited with an audit event. Once paid,
	// their amount must not fall below allocations and settled invoices are locked.
	got.Items[0].UnitPrice = 2000
	if err := store.UpdateInvoiceDraft(ctx, got); err != nil {
		t.Fatal(err)
	}
	var events int
	if err := store.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_events WHERE entity_id=? AND action='unsettled_invoice_updated'", id).Scan(&events); err != nil || events != 1 {
		t.Fatalf("audit events=%d err=%v", events, err)
	}
	payment := domain.Payment{Amount: 1000, Currency: domain.CurrencyToman, PaidAt: time.Now(), Method: "bank"}
	if _, err := store.RegisterPayment(ctx, payment, []domain.PaymentAllocation{{InvoiceID: id, Amount: 1000}}); err != nil {
		t.Fatal(err)
	}
	got.Items[0].UnitPrice = 400
	if err := store.UpdateInvoiceDraft(ctx, got); err == nil {
		t.Fatal("expected reduction below payments to fail")
	}
	balance, err := store.InvoiceBalance(ctx, id)
	if err != nil || balance != 3000 {
		t.Fatalf("balance after failed edit=%d err=%v", balance, err)
	}
	payment.Amount = 3000
	if _, err := store.RegisterPayment(ctx, payment, []domain.PaymentAllocation{{InvoiceID: id, Amount: 3000}}); err != nil {
		t.Fatal(err)
	}
	got.Items[0].UnitPrice = 2500
	if err := store.UpdateInvoiceDraft(ctx, got); err == nil {
		t.Fatal("expected settled invoice edits to fail")
	}
}
