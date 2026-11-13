package main

import "testing"

func TestInvoicePercentParsing(t *testing.T) {
	for _, test := range []struct {
		raw  string
		want int64
	}{{"", 0}, {"۹٫۵", 950}, {"٩.٢٥", 925}, {"100", 10000}, {"0.01", 1}} {
		got, err := parseInvoicePercent(test.raw)
		if err != nil || got != test.want {
			t.Errorf("parseInvoicePercent(%q) = %d, %v; want %d", test.raw, got, err, test.want)
		}
	}
	for _, raw := range []string{"100.01", "-1", "1.999", "1,2", "abc", "9223372036854775807", "1."} {
		if _, err := parseInvoicePercent(raw); err == nil {
			t.Errorf("accepted invalid percentage %q", raw)
		}
	}
}

func TestInvoiceAmountParsing(t *testing.T) {
	for _, raw := range []string{"۱۲٬۳۴۵", "12,345", "١٢٣٤٥"} {
		got, err := parseInvoiceAmount(raw)
		if err != nil || got != 12345 {
			t.Errorf("parseInvoiceAmount(%q) = %d, %v", raw, got, err)
		}
	}
	for _, raw := range []string{"-1", "1.5", "9223372036854775808"} {
		if _, err := parseInvoiceAmount(raw); err == nil {
			t.Errorf("accepted invalid amount %q", raw)
		}
	}
}
