package domain

import (
	"errors"
	"fmt"
	"math"
)

type DiscountMode string

const (
	DiscountFixed   DiscountMode = "fixed"
	DiscountPercent DiscountMode = "percent"
)

type Discount struct {
	Mode  DiscountMode
	Value Money // fixed amount, or basis points for percentage (100 = 1%)
}

type FinancialTotals struct {
	Subtotal          Money
	ItemDiscount      Money
	InvoiceDiscount   Money
	Tax               Money
	Total             Money
	AdditionalCharges Money
	GrandTotal        Money
	TotalQuantity     int64
}

// CalculateFinancials applies line discounts first, then invoice discount, then tax.
// Percentage values and taxRateBPS are expressed in basis points (100 = 1%).
// All amounts use integer arithmetic; percentage amounts round half-up to the
// smallest configured currency unit.
func CalculateFinancials(items []InvoiceItem, invoiceDiscount Discount, taxRateBPS int64) (FinancialTotals, error) {
	if len(items) == 0 {
		return FinancialTotals{}, errors.New("فاکتور باید دست‌کم یک قلم داشته باشد")
	}
	if taxRateBPS < 0 || taxRateBPS > 10000 {
		return FinancialTotals{}, errors.New("نرخ مالیات باید بین صفر تا ۱۰۰ درصد باشد")
	}
	var out FinancialTotals
	for _, item := range items {
		if item.Quantity > math.MaxInt64-out.TotalQuantity {
			return FinancialTotals{}, errors.New("سرریز جمع تعداد اقلام")
		}
		out.TotalQuantity += item.Quantity
		gross, err := item.gross()
		if err != nil {
			return FinancialTotals{}, err
		}
		if err := item.Validate(); err != nil {
			return FinancialTotals{}, err
		}
		if err := addMoney(&out.Subtotal, Money(gross)); err != nil {
			return FinancialTotals{}, err
		}
		if err := addMoney(&out.ItemDiscount, item.Discount); err != nil {
			return FinancialTotals{}, err
		}
		net, err := item.LineTotal()
		if err != nil {
			return FinancialTotals{}, err
		}
		if err := addMoney(&out.Total, net); err != nil {
			return FinancialTotals{}, err
		}
	}
	switch invoiceDiscount.Mode {
	case DiscountFixed:
		if invoiceDiscount.Value < 0 || invoiceDiscount.Value > out.Total {
			return FinancialTotals{}, errors.New("تخفیف کلی ثابت خارج از محدوده است")
		}
		out.InvoiceDiscount = invoiceDiscount.Value
	case DiscountPercent:
		if invoiceDiscount.Value < 0 || invoiceDiscount.Value > 10000 {
			return FinancialTotals{}, errors.New("درصد تخفیف باید بین صفر تا ۱۰۰ درصد باشد")
		}
		v, err := multiplyDivideRoundHalfUp(int64(out.Total), int64(invoiceDiscount.Value), 10000)
		if err != nil {
			return FinancialTotals{}, err
		}
		out.InvoiceDiscount = Money(v)
	case "":
		if invoiceDiscount.Value != 0 {
			return FinancialTotals{}, errors.New("نوع تخفیف کلی مشخص نشده است")
		}
	default:
		return FinancialTotals{}, errors.New("نوع تخفیف کلی نامعتبر است")
	}
	taxable := int64(out.Total - out.InvoiceDiscount)
	tax, err := multiplyDivideRoundHalfUp(taxable, taxRateBPS, 10000)
	if err != nil {
		return FinancialTotals{}, err
	}
	out.Tax = Money(tax)
	out.Total = Money(taxable)
	if err := addMoney(&out.Total, out.Tax); err != nil {
		return FinancialTotals{}, err
	}
	out.GrandTotal = out.Total
	return out, nil
}

func CalculateFinancialsWithCharges(items []InvoiceItem, discount Discount, taxRateBPS int64, charges []AdditionalCharge) (FinancialTotals, error) {
	result, err := CalculateFinancials(items, discount, taxRateBPS)
	if err != nil {
		return result, err
	}
	for _, charge := range charges {
		if charge.Amount < 0 {
			return FinancialTotals{}, errors.New("مبلغ هزینهٔ اضافه نمی‌تواند منفی باشد")
		}
		if err := addMoney(&result.AdditionalCharges, charge.Amount); err != nil {
			return FinancialTotals{}, err
		}
	}
	result.GrandTotal = result.Total
	if err := addMoney(&result.GrandTotal, result.AdditionalCharges); err != nil {
		return FinancialTotals{}, err
	}
	return result, nil
}

func addMoney(dst *Money, amount Money) error {
	if amount < 0 || int64(*dst) > math.MaxInt64-int64(amount) {
		return errors.New("سرریز محاسبه مبلغ")
	}
	*dst += amount
	return nil
}

func multiplyDivideRoundHalfUp(a, b, divisor int64) (int64, error) {
	if a < 0 || b < 0 || divisor <= 0 {
		return 0, errors.New("ورودی محاسبه درصد نامعتبر است")
	}
	if a != 0 && b > math.MaxInt64/a {
		// Avoid overflowing a*b by using quotient and remainder.
		q, r := a/divisor, a%divisor
		if q != 0 && b > math.MaxInt64/q {
			return 0, errors.New("سرریز محاسبه درصد")
		}
		base := q * b
		if r != 0 && b > math.MaxInt64/r {
			return 0, errors.New("سرریز محاسبه درصد")
		}
		rem := r * b
		if rem/divisor >= math.MaxInt64-base {
			return 0, errors.New("سرریز محاسبه درصد")
		}
		result := base + rem/divisor
		if rem%divisor >= (divisor+1)/2 {
			result++
		}
		return result, nil
	}
	product := a * b
	result := product / divisor
	if product%divisor >= (divisor+1)/2 {
		result++
	}
	if result < 0 {
		return 0, fmt.Errorf("سرریز محاسبه درصد")
	}
	return result, nil
}

// ConvertCurrency converts only when the conversion is exact; it never silently
// truncates a fraction of a toman. IRR is rial and IRT is toman.
func ConvertCurrency(amount Money, from, to Currency) (Money, error) {
	if amount < 0 {
		return 0, errors.New("مبلغ نمی‌تواند منفی باشد")
	}
	if (from != CurrencyRial && from != CurrencyToman) || (to != CurrencyRial && to != CurrencyToman) {
		return 0, errors.New("واحد پول نامعتبر است")
	}
	if from == to {
		return amount, nil
	}
	if from == CurrencyToman {
		if int64(amount) > math.MaxInt64/10 {
			return 0, errors.New("سرریز تبدیل ریال و تومان")
		}
		return Money(int64(amount) * 10), nil
	}
	if int64(amount)%10 != 0 {
		return 0, errors.New("تبدیل ریال به تومان دقیق نیست؛ مبلغ باید مضرب ۱۰ باشد")
	}
	return Money(int64(amount) / 10), nil
}
