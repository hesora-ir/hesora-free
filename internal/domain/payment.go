package domain

import (
	"errors"
	"time"
)

type Payment struct {
	ID          int64
	ExternalRef string
	Amount      Money
	Currency    Currency
	PaidAt      time.Time
	Method      string
	Note        string
}

type PaymentAllocation struct {
	InvoiceID int64
	Amount    Money
}

type PaymentHistoryAllocation struct {
	InvoiceID     int64
	InvoiceNumber string
	Amount        Money
}

type PaymentHistoryEntry struct {
	Payment        Payment
	Allocations    []PaymentHistoryAllocation
	ReversedAt     time.Time
	ReversalReason string
}

func (p Payment) Validate() error {
	if p.ID < 0 {
		return errors.New("شناسه پرداخت نامعتبر است")
	}
	if p.Amount <= 0 {
		return errors.New("مبلغ پرداخت باید بزرگ‌تر از صفر باشد")
	}
	if p.Currency != CurrencyRial && p.Currency != CurrencyToman {
		return errors.New("واحد پول پرداخت نامعتبر است")
	}
	if p.PaidAt.IsZero() {
		return errors.New("تاریخ پرداخت الزامی است")
	}
	if len(p.ExternalRef) > 200 || len(p.Method) > 100 || len(p.Note) > 2000 {
		return errors.New("اطلاعات پرداخت بیش از حد مجاز است")
	}
	return nil
}

func ValidateAllocations(payment Payment, allocations []PaymentAllocation) error {
	if err := payment.Validate(); err != nil {
		return err
	}
	if len(allocations) == 0 {
		return errors.New("پرداخت باید دست‌کم به یک فاکتور تخصیص یابد")
	}
	var sum int64
	seen := make(map[int64]struct{}, len(allocations))
	for _, a := range allocations {
		if a.InvoiceID <= 0 || a.Amount <= 0 {
			return errors.New("تخصیص پرداخت نامعتبر است")
		}
		if _, ok := seen[a.InvoiceID]; ok {
			return errors.New("برای هر فاکتور فقط یک تخصیص در هر پرداخت مجاز است")
		}
		seen[a.InvoiceID] = struct{}{}
		if int64(a.Amount) > int64(payment.Amount)-sum {
			return errors.New("مجموع تخصیص‌ها از مبلغ پرداخت بیشتر است")
		}
		sum += int64(a.Amount)
	}
	if sum != int64(payment.Amount) {
		return errors.New("مجموع تخصیص‌ها باید با مبلغ پرداخت برابر باشد")
	}
	return nil
}
