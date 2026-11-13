package pdf

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hesora-ir/hesora-free/internal/banking"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hesora-ir/hesora-free/internal/domain"
	invoicehtml "github.com/hesora-ir/hesora-free/internal/render/html"
	"github.com/hesora-ir/hesora-free/internal/textshape"
)

type Options struct {
	FontDirectory string
	FontName      string
}

func ExportInvoice(data invoicehtml.PreviewData, outputPath string, opts Options) error {
	computed, calcErr := domain.CalculateFinancialsWithCharges(data.Document.Items, data.Document.InvoiceDiscount, data.Document.TaxRateBPS, data.Document.AdditionalCharges)
	if calcErr != nil {
		return calcErr
	}
	data.Totals = computed
	if strings.TrimSpace(outputPath) == "" {
		return errors.New("مسیر خروجی PDF الزامی است")
	}
	if err := data.Template.Validate(); err != nil {
		return err
	}
	if len(data.Document.Items) == 0 {
		return errors.New("سند بدون قلم قابل خروجی نیست")
	}
	if strings.TrimSpace(opts.FontDirectory) == "" || strings.TrimSpace(opts.FontName) == "" {
		return errors.New("پوشه و نام فونت فارسی PDF الزامی است")
	}
	if strings.ContainsAny(opts.FontName, "/\\") || strings.Contains(opts.FontName, "..") {
		return errors.New("نام فونت نامعتبر است")
	}
	if info, err := os.Stat(opts.FontDirectory); err != nil || !info.IsDir() {
		return errors.New("پوشه فونت PDF در دسترس نیست")
	}
	fontPath := filepath.Join(opts.FontDirectory, opts.FontName)
	if strings.ToLower(filepath.Ext(opts.FontName)) != ".ttf" {
		return errors.New("فونت PDF باید فایل TTF باشد")
	}
	if info, err := os.Stat(fontPath); err != nil || info.IsDir() {
		return fmt.Errorf("فایل TTF فونت PDF در دسترس نیست: %s", fontPath)
	}

	customer, issuer, err := snapshotParties(data)
	if err != nil {
		return err
	}
	banks, err := bankAccountsForDocument(data)
	if err != nil {
		return err
	}

	return renderInvoice(data, customer, issuer, banks, outputPath, opts)
}

func fa(value string) string {
	shaped, err := textshape.Bidi(value)
	if err != nil {
		return value
	}
	return shaped
}

func parseColor(value string, fallback [3]int) [3]int {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(value) != 6 {
		return fallback
	}
	var r, g, b int
	if _, err := fmt.Sscanf(value, "%02x%02x%02x", &r, &g, &b); err != nil {
		return fallback
	}
	return [3]int{r, g, b}
}

func nonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, strings.TrimSpace(value))
		}
	}
	return out
}

func snapshotParties(data invoicehtml.PreviewData) (invoicehtml.PartySnapshot, invoicehtml.PartySnapshot, error) {
	customer, issuer := data.Customer, data.Issuer
	if strings.TrimSpace(data.Document.CustomerSnapshot) != "" {
		if err := json.Unmarshal([]byte(data.Document.CustomerSnapshot), &customer); err != nil {
			return customer, issuer, fmt.Errorf("snapshot مشتری نامعتبر است: %w", err)
		}
	}
	if strings.TrimSpace(data.Document.IssuerSnapshot) != "" {
		if err := json.Unmarshal([]byte(data.Document.IssuerSnapshot), &issuer); err != nil {
			return customer, issuer, fmt.Errorf("snapshot صادرکننده نامعتبر است: %w", err)
		}
	}
	return customer, issuer, nil
}

func bankAccountsForDocument(data invoicehtml.PreviewData) ([]domain.BankAccount, error) {
	if strings.TrimSpace(data.Document.BankAccountsSnapshot) == "" {
		return data.BankAccounts, nil
	}
	var selected []domain.IssuerBankDetails
	if err := json.Unmarshal([]byte(data.Document.BankAccountsSnapshot), &selected); err != nil {
		return nil, fmt.Errorf("snapshot حساب بانکی نامعتبر است: %w", err)
	}
	result := make([]domain.BankAccount, 0, len(selected))
	for _, a := range selected {
		result = append(result, domain.BankAccount{BankName: banking.ResolveBankName(a.BankName, a.IBAN, a.CardNumber), AccountNumber: a.AccountNumber, CardNumber: a.CardNumber, IBAN: banking.DisplayIBAN(a.IBAN)})
	}
	return result, nil
}

func partyLine(p invoicehtml.PartySnapshot) string {
	return strings.TrimSpace(strings.Join(nonEmpty([]string{first(p.Name, p.LegalName), p.Phone, p.Address}), "  |  "))
}
func amount(m domain.Money) string { return fmt.Sprintf("%d", m) }
func moneyLabel(m domain.Money) string {
	s := fmt.Sprintf("%d", m)
	start := 0
	if strings.HasPrefix(s, "-") {
		start = 1
	}
	for i := len(s) - 3; i > start; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return faDigits(s)
}
func faDigits(value string) string {
	return strings.NewReplacer("0", "۰", "1", "۱", "2", "۲", "3", "۳", "4", "۴", "5", "۵", "6", "۶", "7", "۷", "8", "۸", "9", "۹").Replace(value)
}
func jalaliDate(t time.Time) string {
	d, err := domain.GregorianToJalali(t.Year(), int(t.Month()), t.Day())
	if err != nil {
		return faDigits(t.Format("2006/01/02"))
	}
	return faDigits(fmt.Sprintf("%04d/%02d/%02d", d.Year, d.Month, d.Day))
}
func currencyName(c domain.Currency) string {
	if c == domain.CurrencyRial {
		return "ریال"
	}
	return "تومان"
}
func documentLabel(kind string) string {
	if strings.EqualFold(strings.TrimSpace(kind), "proforma") {
		return "پیش فاکتور"
	}
	return "فاکتور"
}
func documentLabelLatin(kind string) string {
	if strings.EqualFold(strings.TrimSpace(kind), "proforma") {
		return "PROFORMA"
	}
	return "INVOICE"
}
func first(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return "فاکتور"
}
func parent(path string) string {
	i := strings.LastIndexAny(path, "/\\")
	if i < 0 {
		return "."
	}
	if i == 0 {
		return path[:1]
	}
	return path[:i]
}
