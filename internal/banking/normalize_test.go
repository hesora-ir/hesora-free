package banking

import (
	"strings"
	"testing"
)

func TestIranianIBANNormalizationAndChecksum(t *testing.T) {
	const canonical = "IR820540102680020817909002"
	for _, input := range []string{canonical, "ir82 0540 1026 8002 0817 9090 02", canonical[2:], "\u202aIR۸۲-۰۵۴۰-۱۰۲۶-۸۰۰۲-۰۸۱۷-۹۰۹۰-۰۲\u202c", "IR٨٢٠٥٤٠١٠٢٦٨٠٠٢٠٨١٧٩٠٩٠٠٢"} {
		got := ValidateIBAN(input)
		if !got.Valid || got.Normalized != canonical || got.Bank == nil || got.Bank.Name != "بانک پارسیان" {
			t.Fatalf("ValidateIBAN(%q) = %+v", input, got)
		}
	}
	for _, input := range []string{"", "IR82", "DE" + canonical[2:], canonical[:25] + "3", "IR00" + canonical[4:], "IR99" + canonical[4:], "IR82x" + canonical[5:]} {
		if ValidateIBAN(input).Valid {
			t.Errorf("accepted invalid IBAN %q", input)
		}
	}
	// A checksum-valid unknown bank must not be silently assigned a bank name.
	if got := ValidateIBAN("IR820000102680020817909002"); got.Valid || got.Bank != nil {
		t.Fatalf("unknown bank accepted: %+v", got)
	}
}

func TestHistoricalBankNameAndLegacySnapshotFallback(t *testing.T) {
	const iban = "IR820540102680020817909002"
	if got := ResolveBankName("بانک زمان صدور", iban, ""); got != "بانک زمان صدور" {
		t.Fatal(got)
	}
	if got := ResolveBankName("", iban, ""); got != "بانک پارسیان" {
		t.Fatal(got)
	}
	if got := ResolveBankName("", "invalid", "0000000000000000"); got != "" {
		t.Fatal(got)
	}
	for _, digit := range "0123456789" {
		if ValidateCard(strings.Repeat(string(digit), 16)).Valid {
			t.Fatal("repeated-digit card accepted")
		}
	}
}
