package domain

import (
	"errors"
	"strings"
	"time"
)

type Expense struct {
	ID        int64
	Amount    Money
	Currency  Currency
	Date      time.Time
	Category  string
	Note      string
	CreatedAt time.Time
}

func (e Expense) Validate() error {
	if e.Amount <= 0 || (e.Currency != CurrencyRial && e.Currency != CurrencyToman) || e.Date.IsZero() || strings.TrimSpace(e.Category) == "" || len(e.Category) > 120 || len(e.Note) > 2000 {
		return errors.New("اطلاعات هزینه معتبر نیست")
	}
	return nil
}
