package domain

import (
	"math"
	"testing"
)

func TestCalculateTotalsUsesIntegerArithmetic(t *testing.T) {
	invoice := Invoice{
		Number: "INV-1", Currency: CurrencyToman, Status: InvoiceDraft,
		Items: []InvoiceItem{
			{Description: "کالا", Quantity: 3, UnitPrice: 1250, Discount: 250},
			{Description: "خدمت", Quantity: 2, UnitPrice: 500, Discount: 0},
		},
	}
	got, err := invoice.CalculateTotals()
	if err != nil {
		t.Fatal(err)
	}
	if got.Subtotal != 4750 || got.Discount != 250 || got.Total != 4500 {
		t.Fatalf("unexpected totals: %+v", got)
	}
}

func TestInvoiceItemRejectsOverflow(t *testing.T) {
	item := InvoiceItem{Description: "کالا", Quantity: 2, UnitPrice: Money(math.MaxInt64)}
	if _, err := item.LineTotal(); err == nil {
		t.Fatal("expected overflow error")
	}
}

func TestInvoiceItemRejectsDiscountAboveGross(t *testing.T) {
	item := InvoiceItem{Description: "کالا", Quantity: 1, UnitPrice: 100, Discount: 101}
	if err := item.Validate(); err != ErrInvalidDiscount {
		t.Fatalf("expected ErrInvalidDiscount, got %v", err)
	}
}

func TestInvoiceRequiresNumberAndCurrency(t *testing.T) {
	if err := (Invoice{Currency: CurrencyRial, Status: InvoiceDraft}).Validate(); err == nil {
		t.Fatal("expected missing invoice number to fail")
	}
	if err := (Invoice{Number: "1", Currency: "USD", Status: InvoiceDraft}).Validate(); err == nil {
		t.Fatal("expected unsupported currency to fail")
	}
}
