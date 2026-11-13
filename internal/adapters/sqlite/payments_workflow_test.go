package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/hesora-ir/hesora-free/internal/domain"
)

func TestRegisterPaymentAndBalance(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "payments.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	doc := testInvoiceDraft()
	id, _, err := store.CreateInvoiceDraft(ctx, doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.IssueInvoice(ctx, id); err != nil {
		t.Fatal(err)
	}
	paidAt := time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)
	first := domain.Payment{ExternalRef: "receipt-000", Amount: domain.Money(500), Currency: domain.CurrencyToman, PaidAt: paidAt, Method: "bank"}
	if _, err := store.RegisterPayment(ctx, first, []domain.PaymentAllocation{{InvoiceID: id, Amount: domain.Money(500)}}); err != nil {
		t.Fatal(err)
	}

	payment := domain.Payment{ExternalRef: "receipt-001", Amount: domain.Money(700), Currency: domain.CurrencyToman, PaidAt: paidAt, Method: "bank"}
	// This passes domain validation but exceeds the invoice's remaining balance.
	if _, err := store.RegisterPayment(ctx, payment, []domain.PaymentAllocation{{InvoiceID: id, Amount: domain.Money(700)}}); err == nil {
		t.Fatal("expected over-allocation to fail")
	}
	// The failed transaction must roll back the payment row so the same reference can be retried.
	payment.Amount = domain.Money(400)
	paymentID, err := store.RegisterPayment(ctx, payment, []domain.PaymentAllocation{{InvoiceID: id, Amount: domain.Money(400)}})
	if err != nil {
		t.Fatal(err)
	}
	if paymentID <= 0 {
		t.Fatalf("payment id = %d", paymentID)
	}
	balance, err := store.InvoiceBalance(ctx, id)
	if err != nil || balance != domain.Money(100) {
		t.Fatalf("balance=%v err=%v", balance, err)
	}
	if _, err := store.RegisterPayment(ctx, payment, []domain.PaymentAllocation{{InvoiceID: id, Amount: domain.Money(400)}}); err == nil {
		t.Fatal("expected duplicate external reference to fail")
	}
	balance, err = store.InvoiceBalance(ctx, id)
	if err != nil || balance != domain.Money(100) {
		t.Fatalf("balance after duplicate=%v err=%v", balance, err)
	}
}

func TestRegisterPaymentRejectsCurrencyMismatchAndUnissuedInvoice(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "payment-validation.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	doc := testInvoiceDraft()
	id, _, err := store.CreateInvoiceDraft(ctx, doc)
	if err != nil {
		t.Fatal(err)
	}
	p := domain.Payment{ExternalRef: "x", Amount: domain.Money(100), Currency: domain.CurrencyRial, PaidAt: time.Now().UTC()}
	if _, err := store.RegisterPayment(ctx, p, []domain.PaymentAllocation{{InvoiceID: id, Amount: domain.Money(100)}}); err == nil {
		t.Fatal("expected unissued invoice allocation to fail")
	}
	if err := store.IssueInvoice(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RegisterPayment(ctx, p, []domain.PaymentAllocation{{InvoiceID: id, Amount: domain.Money(100)}}); err == nil {
		t.Fatal("expected currency mismatch to fail")
	}
}
