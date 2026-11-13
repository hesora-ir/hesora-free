package excel

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestLegacyIBANAndBankNameInExcel(t *testing.T) {
	data := sampleData()
	data.Document.BankAccountsSnapshot = `[{"IBAN":"IR۸۲ ۰۵۴۰ ۱۰۲۶ ۸۰۰۲ ۰۸۱۷ ۹۰۹۰ ۰۲"}]`
	path := filepath.Join(t.TempDir(), "bank.xlsx")
	if err := ExportInvoice(data, path); err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	rows, err := book.GetRows("فاکتور")
	if err != nil {
		t.Fatal(err)
	}
	var values []string
	for _, row := range rows {
		values = append(values, row...)
	}
	all := strings.Join(values, "\n")
	if !strings.Contains(all, "بانک پارسیان") || !strings.Contains(all, "IR820540102680020817909002") {
		t.Fatal("missing normalized IBAN or bank name")
	}
}
