package domain

import "testing"

func TestCalculateFinancials(t *testing.T) {
	items := []InvoiceItem{
		{Description: "قلم اول", Quantity: 2, UnitPrice: Money(1000), Discount: Money(100)},
		{Description: "قلم دوم", Quantity: 1, UnitPrice: Money(500)},
	}
	got, err := CalculateFinancials(items, Discount{Mode: DiscountPercent, Value: 1000}, 900) // 10% discount, 9% tax
	if err != nil {
		t.Fatal(err)
	}
	if got.Subtotal != 2500 || got.ItemDiscount != 100 || got.InvoiceDiscount != 240 || got.Tax != 194 || got.Total != 2354 {
		t.Fatalf("unexpected totals: %+v", got)
	}
}

func TestCalculateFinancialsRejectsInvalidAndOverflow(t *testing.T) {
	_, err := CalculateFinancials(nil, Discount{}, 0)
	if err == nil {
		t.Fatal("expected empty invoice error")
	}
	_, err = CalculateFinancials([]InvoiceItem{{Description: "x", Quantity: 1, UnitPrice: Money(1)}}, Discount{Mode: DiscountPercent, Value: 10001}, 0)
	if err == nil {
		t.Fatal("expected invalid discount error")
	}
	_, err = CalculateFinancials([]InvoiceItem{{Description: "x", Quantity: 2, UnitPrice: Money(maxInt64)}}, Discount{}, 0)
	if err == nil {
		t.Fatal("expected overflow error")
	}
}

func TestConvertCurrencyExactOnly(t *testing.T) {
	got, err := ConvertCurrency(Money(123), CurrencyToman, CurrencyRial)
	if err != nil || got != 1230 {
		t.Fatalf("got %v, err %v", got, err)
	}
	if _, err := ConvertCurrency(Money(123), CurrencyRial, CurrencyToman); err == nil {
		t.Fatal("expected inexact conversion error")
	}
	got, err = ConvertCurrency(Money(1230), CurrencyRial, CurrencyToman)
	if err != nil || got != 123 {
		t.Fatalf("got %v, err %v", got, err)
	}
}
