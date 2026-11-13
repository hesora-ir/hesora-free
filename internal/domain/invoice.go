package domain

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvalidInvoiceID = errors.New("شناسه فاکتور نامعتبر است")
	ErrInvalidItem      = errors.New("قلم فاکتور نامعتبر است")
	ErrInvalidQuantity  = errors.New("تعداد باید بزرگ‌تر از صفر باشد")
	ErrInvalidPrice     = errors.New("قیمت نمی‌تواند منفی باشد")
	ErrInvalidDiscount  = errors.New("تخفیف خارج از محدوده است")
)

const maxInt64 = int64(^uint64(0) >> 1)

// Money stores the smallest configured currency unit as an integer.
// The application must explicitly record whether that unit is rial or toman.
type Money int64

type Currency string

const (
	CurrencyRial  Currency = "IRR"
	CurrencyToman Currency = "IRT"
)

type InvoiceStatus string

const (
	InvoiceDraft  InvoiceStatus = "draft"
	InvoiceIssued InvoiceStatus = "issued"
	InvoiceVoid   InvoiceStatus = "void"
)

type InvoiceItem struct {
	ID          int64
	Description string
	Unit        string
	Quantity    int64 // scaled integer quantity; scale is defined by the UI/product unit
	UnitPrice   Money
	Discount    Money // fixed discount per line, in the invoice currency
}

type Invoice struct {
	ID       int64
	Number   string
	Currency Currency
	Status   InvoiceStatus
	Items    []InvoiceItem
}

type Totals struct {
	Subtotal Money
	Discount Money
	Total    Money
}

func (i InvoiceItem) gross() (int64, error) {
	if i.Quantity <= 0 {
		return 0, ErrInvalidQuantity
	}
	if i.UnitPrice < 0 {
		return 0, ErrInvalidPrice
	}
	price := int64(i.UnitPrice)
	if price != 0 && i.Quantity > maxInt64/price {
		return 0, errors.New("سرریز محاسبه مبلغ قلم")
	}
	return price * i.Quantity, nil
}

func (i InvoiceItem) Validate() error {
	if strings.TrimSpace(i.Description) == "" {
		return fmt.Errorf("%w: شرح خالی است", ErrInvalidItem)
	}
	if len(i.Unit) > 50 {
		return fmt.Errorf("%w: واحد بیش از حد مجاز است", ErrInvalidItem)
	}
	gross, err := i.gross()
	if err != nil {
		return err
	}
	if i.Discount < 0 || int64(i.Discount) > gross {
		return ErrInvalidDiscount
	}
	return nil
}

// LineTotal uses integer arithmetic only. Quantity is an integer scale chosen by the caller.
func (i InvoiceItem) LineTotal() (Money, error) {
	if err := i.Validate(); err != nil {
		return 0, err
	}
	gross, _ := i.gross()
	return Money(gross - int64(i.Discount)), nil
}

func (i Invoice) Validate() error {
	if i.ID < 0 {
		return ErrInvalidInvoiceID
	}
	if strings.TrimSpace(i.Number) == "" {
		return errors.New("شماره فاکتور الزامی است")
	}
	if i.Currency != CurrencyRial && i.Currency != CurrencyToman {
		return errors.New("واحد پول نامعتبر است")
	}
	if i.Status != InvoiceDraft && i.Status != InvoiceIssued && i.Status != InvoiceVoid {
		return errors.New("وضعیت فاکتور نامعتبر است")
	}
	for _, item := range i.Items {
		if err := item.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (i Invoice) CalculateTotals() (Totals, error) {
	if err := i.Validate(); err != nil {
		return Totals{}, err
	}
	var totals Totals
	for _, item := range i.Items {
		gross, _ := item.gross()
		if int64(totals.Subtotal) > maxInt64-gross {
			return Totals{}, errors.New("سرریز جمع فاکتور")
		}
		if int64(totals.Discount) > maxInt64-int64(item.Discount) {
			return Totals{}, errors.New("سرریز جمع تخفیف")
		}
		net := gross - int64(item.Discount)
		if int64(totals.Total) > maxInt64-net {
			return Totals{}, errors.New("سرریز مبلغ نهایی فاکتور")
		}
		totals.Subtotal += Money(gross)
		totals.Discount += item.Discount
		totals.Total += Money(net)
	}
	return totals, nil
}
