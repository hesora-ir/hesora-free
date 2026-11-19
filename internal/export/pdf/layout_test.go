package pdf

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/hesora-ir/hesora-free/internal/domain"
	invoicehtml "github.com/hesora-ir/hesora-free/internal/render/html"
)

func TestLogicalWrappingPreservesParagraphOrder(t *testing.T) {
	measure := func(value string) float64 { return float64(utf8.RuneCountInString(value)) }
	got := wrapLogicalText("سطر اول\nسطر دوم\nسطر سوم", 20, measure)
	if strings.Join(got, "|") != "سطر اول|سطر دوم|سطر سوم" {
		t.Fatalf("paragraphs reordered: %q", got)
	}
	for _, value := range []string{"شرح طولانی یک خدمت همراه با مشخصات فنی", "INV-ABCDEFGHIJKLMNOPQRSTUVWXYZ-1234567890", "ایمیل: info@example.com\r\nتلفن: 09121234567"} {
		lines := wrapLogicalText(value, 12, measure)
		for _, line := range lines {
			if measure(line) > 12 {
				t.Fatalf("overflow: %q", line)
			}
		}
		compact := func(value string) string { return strings.Join(strings.Fields(value), "") }
		if compact(strings.Join(lines, "")) != compact(value) {
			t.Fatalf("text lost: %q -> %q", value, lines)
		}
	}
}

// Real renderer fixtures cover a normal invoice, long content crossing page
// boundaries, and Letter landscape. Set PDF_REVIEW_DIR to retain the artifacts.
func TestInvoiceLayoutFixtures(t *testing.T) {
	output := t.TempDir()
	if path := os.Getenv("PDF_REVIEW_DIR"); path != "" {
		output = path
	}
	data := invoicehtml.PreviewData{
		Document: domain.InvoiceDocument{Number: "INV-000001", Kind: "invoice", IssueDate: time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC), Currency: domain.CurrencyToman,
			Items:           []domain.InvoiceItem{{Description: "طراحی و پیاده‌سازی وب‌سایت", Quantity: 1, Unit: "پروژه", UnitPrice: 20000000}, {Description: "پشتیبانی و خدمات نرم‌افزاری", Quantity: 1, Unit: "بسته", UnitPrice: 20000000}},
			InvoiceDiscount: domain.Discount{Mode: domain.DiscountFixed, Value: 20000000}, TaxRateBPS: 1000,
			Note: "این فاکتور تا ۲۴ ساعت معتبر است.\nبعد از پرداخت، اطلاعات پرداختی ارسال شود.\nتحویل سفارش پس از تأیید نهایی انجام می‌شود."},
		Issuer:       invoicehtml.PartySnapshot{Name: "استودیو طراحی آبان", Phone: "09121234567", Email: "hello@example.com", Address: "تهران، خیابان آزادی، خیابان استاد معین، مجتمع تجاری، طبقه دوم، واحد ۱۲"},
		Customer:     invoicehtml.PartySnapshot{Name: "شرکت توسعه پارس", Phone: "02112345678", Email: "finance@example.com", Address: "تهران، میدان ونک، خیابان ملاصدرا، پلاک ۲۴"},
		BankAccounts: []domain.BankAccount{{BankName: "ملت", AccountHolder: "استودیو طراحی آبان", AccountNumber: "123456789012", CardNumber: "6104331234567890", IBAN: "IR120170000000123456789012"}},
		Template:     domain.DocumentTemplate{Name: "پیش‌فرض", Prefix: "FACT-", ThankYou: "از خرید شما سپاسگزاریم", ShowSeal: true, ShowSignature: true, PageSize: "A4", Orientation: "portrait", FontFamily: "IRANSansX", FontSize: 10, PrimaryColor: "#285082", AccentColor: "#4777A8", ShowTax: true, ShowBankAccounts: true},
	}
	var err error
	data.Totals, err = domain.CalculateFinancials(data.Document.Items, data.Document.InvoiceDiscount, data.Document.TaxRateBPS)
	if err != nil {
		t.Fatal(err)
	}
	opts := Options{FontDirectory: "../../../assets/font/iransans", FontName: "IRANSansX-Regular.ttf"}
	if err := ExportInvoice(data, filepath.Join(output, "invoice-preview.pdf"), opts); err != nil {
		t.Fatal(err)
	}
	stress := data
	stress.Document.Items = nil
	for i := 0; i < 48; i++ {
		stress.Document.Items = append(stress.Document.Items, domain.InvoiceItem{Description: "خدمات توسعه نرم‌افزار و پشتیبانی API / نسخه Enterprise همراه با بررسی و رفع خطاهای فنی", Quantity: 2, Unit: "ساعت", UnitPrice: 1234567, Discount: 10000})
	}
	stress.Document.Items[12].Description = strings.Repeat("شرح بسیار طولانی خدمات و جزئیات سفارش برای بررسی ادامه ردیف در صفحه بعد. ", 180)
	stress.Document.Note = strings.Repeat("شرایط تحویل و پرداخت باید پیش از شروع پروژه به تأیید طرفین برسد.\n", 70)
	stress.Issuer.Address = strings.Repeat("تهران، خیابان آزادی، طبقه دوم، واحد دوازده؛ ", 8)
	stress.Totals, err = domain.CalculateFinancials(stress.Document.Items, stress.Document.InvoiceDiscount, stress.Document.TaxRateBPS)
	if err != nil {
		t.Fatal(err)
	}
	if err := ExportInvoice(stress, filepath.Join(output, "invoice-multipage.pdf"), opts); err != nil {
		t.Fatal(err)
	}
	pdfBytes, err := os.ReadFile(filepath.Join(output, "invoice-multipage.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	pageCount := bytes.Count(pdfBytes, []byte("/Type /Page")) - bytes.Count(pdfBytes, []byte("/Type /Pages"))
	if pageCount < 2 {
		t.Fatalf("long invoice should render across multiple pages, got %d", pageCount)
	}
	data.Template.PageSize, data.Template.Orientation = "Letter", "landscape"
	data.Document.Kind = "proforma"
	if err := ExportInvoice(data, filepath.Join(output, "invoice-landscape.pdf"), opts); err != nil {
		t.Fatal(err)
	}
}

func TestPersianDateAndDigitsForPDF(t *testing.T) {
	if got, want := jalaliDate(time.Date(2024, 3, 20, 0, 0, 0, 0, time.UTC)), "۱۴۰۳/۰۱/۰۱"; got != want {
		t.Fatalf("Jalali PDF date=%q want=%q", got, want)
	}
	if got, want := faDigits("INV-2026/11/20 123,456"), "INV-۲۰۲۶/۱۱/۲۰ ۱۲۳,۴۵۶"; got != want {
		t.Fatalf("localized PDF digits=%q want=%q", got, want)
	}
}
