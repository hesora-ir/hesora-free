package pdf

import (
	"github.com/hesora-ir/hesora-free/internal/domain"
	invoicehtml "github.com/hesora-ir/hesora-free/internal/render/html"
	"testing"
)

func TestLegacyBankSnapshotNormalization(t *testing.T) {
	accounts, err := bankAccountsForDocument(invoicehtml.PreviewData{Document: domain.InvoiceDocument{BankAccountsSnapshot: `[{"IBAN":"IR۸۲ ۰۵۴۰ ۱۰۲۶ ۸۰۰۲ ۰۸۱۷ ۹۰۹۰ ۰۲"}]`}})
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || accounts[0].IBAN != "IR820540102680020817909002" || accounts[0].BankName != "بانک پارسیان" {
		t.Fatalf("accounts=%+v", accounts)
	}
}
