package html

import (
	"strings"
	"testing"
	"time"

	"github.com/hesora-ir/hesora-free/internal/domain"
)

func previewFixture() PreviewData {
	doc := domain.InvoiceDocument{Number: "INV-100", Kind: "invoice", Currency: domain.CurrencyToman, IssueDate: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), CustomerSnapshot: `{"name":"مشتری <آزمایشی>"}`, IssuerSnapshot: `{"name":"شرکت نمونه"}`, Items: []domain.InvoiceItem{{Description: "خدمت <ویژه>", Unit: "ساعت", Quantity: 2, UnitPrice: domain.Money(500)}}}
	t := domain.DocumentTemplate{Name: "قالب", PageSize: "A4", Orientation: "portrait", FontFamily: "Tahoma", FontSize: 10, PrimaryColor: "#123456", AccentColor: "#456789", ShowTax: true, ShowBankAccounts: true}
	totals, _ := domain.CalculateFinancials(doc.Items, domain.Discount{Mode: domain.DiscountFixed}, 0)
	return PreviewData{Document: doc, Totals: totals, Template: t, BankAccounts: []domain.BankAccount{{BankName: "بانک نمونه", AccountHolder: "شرکت نمونه", AccountNumber: "123"}}}
}

func TestRenderInvoicePreview(t *testing.T) {
	got, err := RenderInvoicePreview(previewFixture())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`lang="fa" dir="rtl"`, "INV-100", "&lt;ویژه&gt;", "A4 portrait", "شرکت نمونه", "width:100%;", "width:45%;"} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered HTML missing %q", want)
		}
	}
	if strings.Contains(got, "خدمت <ویژه>") {
		t.Fatal("unescaped user content in HTML")
	}
}

func TestRenderInvoicePreviewRejectsInvalidTemplateAndSnapshot(t *testing.T) {
	data := previewFixture()
	data.Template.PrimaryColor = "url(javascript:alert(1))"
	if _, err := RenderInvoicePreview(data); err == nil {
		t.Fatal("expected invalid template error")
	}
	data = previewFixture()
	data.Document.CustomerSnapshot = "{"
	if _, err := RenderInvoicePreview(data); err == nil {
		t.Fatal("expected malformed snapshot error")
	}
}

func TestRenderInvoicePreviewUsesPerInvoiceSnapshot(t *testing.T) {
	data := previewFixture()
	data.Customer = PartySnapshot{Name: "نام فعلی مشتری", Phone: "000"}
	data.Issuer = PartySnapshot{Name: "نام فعلی صادرکننده"}
	data.Document.CustomerSnapshot = `{"name":"مشتری زمان صدور","phone":"111"}`
	data.Document.IssuerSnapshot = `{"name":"صادرکننده زمان صدور"}`
	data.Document.BankAccountsSnapshot = `[{"CardNumber":"6037","AccountNumber":"123456","IBAN":"IR0001"}]`
	data.BankAccounts = []domain.BankAccount{{BankName: "بانک فعلی", AccountHolder: "مالک فعلی", AccountNumber: "999"}}
	got, err := RenderInvoicePreview(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"مشتری زمان صدور", "صادرکننده زمان صدور", "123456", "6037", "IR0001"} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered HTML missing stored snapshot field %q", want)
		}
	}
	for _, unexpected := range []string{"نام فعلی مشتری", "نام فعلی صادرکننده", "بانک فعلی", "مالک فعلی", "999"} {
		if strings.Contains(got, unexpected) {
			t.Errorf("rendered HTML used mutable profile value %q", unexpected)
		}
	}
}

func TestRenderInvoicePreviewRejectsMalformedBankSnapshot(t *testing.T) {
	data := previewFixture()
	data.Document.BankAccountsSnapshot = "{"
	if _, err := RenderInvoicePreview(data); err == nil {
		t.Fatal("expected malformed bank snapshot error")
	}
}

func TestRenderInvoicePreviewRespectsBankVisibilitySetting(t *testing.T) {
	data := previewFixture()
	data.Template.ShowBankAccounts = false
	data.BankAccounts = []domain.BankAccount{{
		BankName:      "بانک مخفی آزمایشی",
		AccountHolder: "مالک حساب آزمایشی",
		AccountNumber: "fallback-account",
		CardNumber:    "fallback-card",
		IBAN:          "fallback-iban",
	}}
	data.Document.BankAccountsSnapshot = `[{"AccountNumber":"masked-account","CardNumber":"masked-card","IBAN":"masked-iban"}]`

	got, err := RenderInvoicePreview(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{
		"masked-account", "masked-card", "masked-iban",
		"بانک مخفی آزمایشی", "مالک حساب آزمایشی",
		"fallback-account", "fallback-card", "fallback-iban",
	} {
		if strings.Contains(got, value) {
			t.Errorf("bank details leaked while bank display is disabled: %q", value)
		}
	}
	// The issuer's name is also used in the normal party section and must remain visible.
	if !strings.Contains(got, "شرکت نمونه") {
		t.Error("issuer name unexpectedly missing from the invoice preview")
	}
}
