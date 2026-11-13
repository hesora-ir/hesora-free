package html

import (
	"strings"
	"testing"
)

func TestLegacyBankSnapshotResolvesIssuerBankName(t *testing.T) {
	data := previewFixture()
	data.Document.BankAccountsSnapshot = `[{"IBAN":"\u202aIR۸۲-۰۵۴۰-۱۰۲۶-۸۰۰۲-۰۸۱۷-۹۰۹۰-۰۲\u202c"}]`
	got, err := RenderInvoicePreview(data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "بانک پارسیان") || !strings.Contains(got, `<bdi dir="ltr">IR820540102680020817909002</bdi>`) {
		t.Fatal("bank or LTR IBAN missing")
	}
}
