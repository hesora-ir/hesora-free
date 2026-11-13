package domain

import (
	"testing"
	"time"
)

func validPaymentForTest() Payment {
	return Payment{
		ID:       1,
		Amount:   Money(1500),
		Currency: CurrencyToman,
		PaidAt:   time.Date(2026, time.October, 25, 12, 0, 0, 0, time.UTC),
		Method:   "bank transfer",
	}
}

func TestPaymentValidate(t *testing.T) {
	if err := validPaymentForTest().Validate(); err != nil {
		t.Fatalf("valid payment rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Payment)
	}{
		{name: "negative ID", mutate: func(p *Payment) { p.ID = -1 }},
		{name: "zero amount", mutate: func(p *Payment) { p.Amount = 0 }},
		{name: "invalid currency", mutate: func(p *Payment) { p.Currency = "USD" }},
		{name: "missing paid date", mutate: func(p *Payment) { p.PaidAt = time.Time{} }},
		{name: "overlong method", mutate: func(p *Payment) { p.Method = string(make([]byte, 101)) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payment := validPaymentForTest()
			tt.mutate(&payment)
			if err := payment.Validate(); err == nil {
				t.Fatal("expected invalid payment to be rejected")
			}
		})
	}
}

func TestValidateAllocations(t *testing.T) {
	payment := validPaymentForTest()
	if err := ValidateAllocations(payment, []PaymentAllocation{
		{InvoiceID: 10, Amount: Money(500)},
		{InvoiceID: 11, Amount: Money(1000)},
	}); err != nil {
		t.Fatalf("valid allocations rejected: %v", err)
	}

	tests := []struct {
		name        string
		allocations []PaymentAllocation
	}{
		{name: "empty", allocations: nil},
		{
			name: "sum below payment",
			allocations: []PaymentAllocation{
				{InvoiceID: 10, Amount: Money(1000)},
			},
		},
		{
			name: "sum above payment",
			allocations: []PaymentAllocation{
				{InvoiceID: 10, Amount: Money(1600)},
			},
		},
		{
			name: "duplicate invoice",
			allocations: []PaymentAllocation{
				{InvoiceID: 10, Amount: Money(500)},
				{InvoiceID: 10, Amount: Money(1000)},
			},
		},
		{
			name: "invalid invoice ID",
			allocations: []PaymentAllocation{
				{InvoiceID: 0, Amount: Money(1500)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateAllocations(payment, tt.allocations); err == nil {
				t.Fatal("expected invalid allocations to be rejected")
			}
		})
	}
}
