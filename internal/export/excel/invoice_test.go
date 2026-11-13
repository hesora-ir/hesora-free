package excel

import (
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/hesora-ir/hesora-free/internal/domain"
	invoicehtml "github.com/hesora-ir/hesora-free/internal/render/html"
	"github.com/xuri/excelize/v2"
)

func sampleData() invoicehtml.PreviewData {
	return invoicehtml.PreviewData{
		Document:     domain.InvoiceDocument{Number: "INV-26-001", Kind: "invoice", Currency: domain.CurrencyToman, IssueDate: time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC), CustomerSnapshot: `{"name":"مشتری زمان صدور","phone":"111","address":"نشانی ثابت"}`, IssuerSnapshot: `{"name":"صادرکننده زمان صدور","phone":"222","address":"نشانی صادرکننده"}`, BankAccountsSnapshot: `[{"AccountNumber":"123456","CardNumber":"6037","IBAN":"IR0001"}]`, Note: "شرایط ثبت‌شده سند", Items: []domain.InvoiceItem{{Description: "خدمت", Unit: "عدد", Quantity: 2, UnitPrice: 1500}}},
		Customer:     invoicehtml.PartySnapshot{Name: "مشتری فعلی", Phone: "000", Address: "نشانی فعلی"},
		Issuer:       invoicehtml.PartySnapshot{Name: "صادرکننده فعلی", Phone: "999", Address: "نشانی فعلی صادرکننده"},
		BankAccounts: []domain.BankAccount{{BankName: "بانک فعلی", AccountHolder: "مالک فعلی", AccountNumber: "999"}},
		Template:     domain.DocumentTemplate{Name: "پیش‌فرض", PageSize: "A4", Orientation: "portrait", FontFamily: "Tahoma", FontSize: 10, PrimaryColor: "#285082", AccentColor: "#4777A8", ShowBankAccounts: true},
		Totals:       domain.FinancialTotals{Subtotal: 3000, Total: 3000},
	}
}

func TestExportInvoiceWritesNumericAmountsAndDocumentSnapshots(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invoice.xlsx")
	if err := ExportInvoice(sampleData(), path); err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	for cell, want := range map[string]string{"A1": "صادرکننده زمان صدور", "D5": "INV-26-001", "A10": "مشخصات صادرکننده", "A11": "صادرکننده زمان صدور", "E11": "مشتری زمان صدور", "A12": "222", "E12": "111", "A14": "نشانی صادرکننده", "E14": "نشانی ثابت", "B17": "خدمت", "E17": "1500", "G17": "3000", "A23": "به حروف: سه هزار تومان", "A25": "توضیحات و شرایط", "A26": "شرایط ثبت‌شده سند", "A28": "اطلاعات پرداخت", "A29": "شماره حساب: 123456  |  کارت: 6037  |  شبا: IR0001"} {
		got, _ := book.GetCellValue("فاکتور", cell, excelize.Options{RawCellValue: true})
		if got != want {
			t.Errorf("%s: expected %q, got %q", cell, want, got)
		}
	}
	// Excelize reports CellTypeUnset for numeric cells whose XML omits the optional
	// type attribute. Accept that representation only when the raw value parses
	// as an integer; explicitly typed string cells must fail this assertion.
	cellType, typeErr := book.GetCellType("فاکتور", "G17")
	rawAmount, valueErr := book.GetCellValue("فاکتور", "G17", excelize.Options{RawCellValue: true})
	_, parseErr := strconv.ParseInt(rawAmount, 10, 64)
	if typeErr != nil || valueErr != nil || parseErr != nil || (cellType != excelize.CellTypeNumber && cellType != excelize.CellTypeUnset) {
		t.Errorf("line amount should be stored as a numeric cell: type=%v value=%q typeErr=%v valueErr=%v parseErr=%v", cellType, rawAmount, typeErr, valueErr, parseErr)
	}
	if got, _ := book.GetCellValue("فاکتور", "A11"); got == "صادرکننده فعلی" {
		t.Fatal("XLSX used mutable issuer data instead of snapshot")
	}
}
func TestExportInvoiceRequiresXLSXExtension(t *testing.T) {
	if err := ExportInvoice(sampleData(), filepath.Join(t.TempDir(), "invoice.csv")); err == nil {
		t.Fatal("expected extension validation error")
	}
}
func TestExportInvoiceRejectsMalformedBankSnapshot(t *testing.T) {
	data := sampleData()
	data.Document.BankAccountsSnapshot = "{"
	if err := ExportInvoice(data, filepath.Join(t.TempDir(), "broken.xlsx")); err == nil {
		t.Fatal("expected malformed bank snapshot error")
	}
}

func TestExportProformaUsesProformaTitleAndThousandsFormat(t *testing.T) {
	data := sampleData()
	data.Document.Kind = "proforma"
	path := filepath.Join(t.TempDir(), "proforma.xlsx")
	if err := ExportInvoice(data, path); err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	if got, _ := book.GetCellValue("فاکتور", "D1"); got != "پیش‌فاکتور" {
		t.Fatalf("unexpected title: %q", got)
	}
	styleID, err := book.GetCellStyle("فاکتور", "G17")
	if err != nil {
		t.Fatal(err)
	}
	if styleID == 0 {
		t.Fatal("amount cell has no number format style")
	}
}
