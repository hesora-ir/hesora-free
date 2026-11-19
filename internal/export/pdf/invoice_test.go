package pdf

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hesora-ir/hesora-free/internal/domain"
	invoicehtml "github.com/hesora-ir/hesora-free/internal/render/html"
)

func TestExportInvoiceWithBundledPersianTTF(t *testing.T) {
	fontDir := filepath.Clean("../../../assets/font/iransans")
	fontName := "IRANSansX-Regular.ttf"
	if _, err := os.Stat(filepath.Join(fontDir, fontName)); err != nil {
		t.Fatalf("bundled Persian TTF missing: %v", err)
	}
	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAAEElEQVR4nGL6//8/IAAA//8GBgMAt2YRIQAAAABJRU5ErkJggg==")
	if err != nil {
		t.Fatal(err)
	}
	imageData, err := domain.EncodeTemplateImage(png, "image/png")
	if err != nil {
		t.Fatal(err)
	}
	data := invoicehtml.PreviewData{
		Document: domain.InvoiceDocument{
			Number: "INV-TTF-1", IssueDate: time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC),
			Currency: domain.CurrencyToman,
			Items:    []domain.InvoiceItem{{Description: "خدمات نرم‌افزاری", Quantity: 1, Unit: "مورد", UnitPrice: 120000}},
		},
		Template: domain.DocumentTemplate{Name: "پیش‌فرض", PageSize: "A4", Orientation: "portrait", FontFamily: "IRANSansX", FontSize: 10, PrimaryColor: "#285082", AccentColor: "#4777A8", ShowLogo: true, LogoData: imageData, ShowSeal: true, SealData: imageData, ShowSignature: true, SignatureData: imageData},
		Totals:   domain.FinancialTotals{Subtotal: 120000, Total: 120000},
	}
	outputPath := filepath.Join(t.TempDir(), "invoice.pdf")
	if err := ExportInvoice(data, outputPath, Options{FontDirectory: fontDir, FontName: fontName}); err != nil {
		t.Fatalf("export with bundled TTF failed: %v", err)
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(content), "%PDF-") {
		t.Fatal("output is not a PDF document")
	}
	if !strings.Contains(string(content), "/Subtype /Image") {
		t.Fatal("PDF output did not embed the template images")
	}
}

func TestExportInvoiceAcceptsTTFNameWithoutMakefontFiles(t *testing.T) {
	fontDir := t.TempDir()
	fontPath := filepath.Join(fontDir, "font.ttf")
	if err := os.WriteFile(fontPath, []byte("not a valid font"), 0644); err != nil {
		t.Fatal(err)
	}
	data := invoicehtml.PreviewData{Document: domain.InvoiceDocument{Items: []domain.InvoiceItem{{Description: "خدمت", Quantity: 1, UnitPrice: 1}}}, Template: domain.DocumentTemplate{Name: "پیش‌فرض", PageSize: "A4", Orientation: "portrait", FontFamily: "Tahoma", FontSize: 10, PrimaryColor: "#285082", AccentColor: "#4777A8"}}
	err := ExportInvoice(data, filepath.Join(fontDir, "invoice.pdf"), Options{FontDirectory: fontDir, FontName: "font.ttf"})
	if err == nil || strings.Contains(err.Error(), ".json") || strings.Contains(err.Error(), ".z") {
		t.Fatalf("expected invalid-TTF parse error, not makefont-file error, got %v", err)
	}
}

func TestExportInvoiceRequiresFont(t *testing.T) {
	data := invoicehtml.PreviewData{
		Document: domain.InvoiceDocument{Number: "INV-1", IssueDate: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), Currency: domain.CurrencyToman, Items: []domain.InvoiceItem{{Description: "خدمت", Quantity: 1, UnitPrice: 100}}},
		Template: domain.DocumentTemplate{Name: "پیش‌فرض", PageSize: "A4", Orientation: "portrait", FontFamily: "Tahoma", FontSize: 10, PrimaryColor: "#285082", AccentColor: "#4777A8"},
		Totals:   domain.FinancialTotals{Subtotal: 100, Total: 100},
	}
	err := ExportInvoice(data, filepath.Join(t.TempDir(), "invoice.pdf"), Options{})
	if err == nil || !strings.Contains(err.Error(), "فونت فارسی") {
		t.Fatalf("expected missing-font validation error, got %v", err)
	}
}
func TestExportInvoiceRejectsInvalidTemplate(t *testing.T) {
	err := ExportInvoice(invoicehtml.PreviewData{Template: domain.DocumentTemplate{}}, filepath.Join(t.TempDir(), "invoice.pdf"), Options{})
	if err == nil {
		t.Fatal("expected invalid template error")
	}
}

func TestDocumentLabelDistinguishesProforma(t *testing.T) {
	if got := documentLabel("proforma"); got != "پیش فاکتور" {
		t.Fatalf("unexpected proforma label: %q", got)
	}
	if got := documentLabel("invoice"); got != "فاکتور" {
		t.Fatalf("unexpected invoice label: %q", got)
	}
	if got := documentLabelLatin("proforma"); got != "PROFORMA" {
		t.Fatalf("unexpected proforma latin label: %q", got)
	}
}
func TestPDFHelpersUsePersistedSnapshots(t *testing.T) {
	data := invoicehtml.PreviewData{
		Document: domain.InvoiceDocument{CustomerSnapshot: `{"name":"مشتری زمان صدور","phone":"111"}`, IssuerSnapshot: `{"name":"صادرکننده زمان صدور","address":"نشانی ثابت"}`, BankAccountsSnapshot: `[{"AccountNumber":"123456","CardNumber":"6037","IBAN":"IR0001"}]`},
		Customer: invoicehtml.PartySnapshot{Name: "مشتری فعلی"}, Issuer: invoicehtml.PartySnapshot{Name: "صادرکننده فعلی"},
		BankAccounts: []domain.BankAccount{{BankName: "بانک فعلی", AccountHolder: "مالک فعلی"}},
	}
	customer, issuer, err := snapshotParties(data)
	if err != nil {
		t.Fatal(err)
	}
	if customer.Name != "مشتری زمان صدور" || customer.Phone != "111" || issuer.Name != "صادرکننده زمان صدور" || issuer.Address != "نشانی ثابت" {
		t.Fatalf("snapshot parties not respected: customer=%+v issuer=%+v", customer, issuer)
	}
	banks, err := bankAccountsForDocument(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(banks) != 1 || banks[0].AccountNumber != "123456" || banks[0].CardNumber != "6037" || banks[0].IBAN != "IR0001" || banks[0].BankName != "" || banks[0].AccountHolder != "" {
		t.Fatalf("snapshot banks not respected: %+v", banks)
	}
	if _, _, err := snapshotParties(invoicehtml.PreviewData{Document: domain.InvoiceDocument{CustomerSnapshot: "{"}}); err == nil {
		t.Fatal("expected malformed party snapshot error")
	}
	if _, err := bankAccountsForDocument(invoicehtml.PreviewData{Document: domain.InvoiceDocument{BankAccountsSnapshot: "{"}}); err == nil {
		t.Fatal("expected malformed bank snapshot error")
	}
}
