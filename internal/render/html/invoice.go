package html

import (
	"encoding/json"
	"fmt"
	"github.com/hesora-ir/hesora-free/internal/banking"
	"html"
	"strings"

	"github.com/hesora-ir/hesora-free/internal/domain"
)

type PartySnapshot struct {
	Name      string `json:"name"`
	LegalName string `json:"legal_name"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	Address   string `json:"address"`
	TaxID     string `json:"tax_id"`
}

type PreviewData struct {
	Document     domain.InvoiceDocument
	Totals       domain.FinancialTotals
	Customer     PartySnapshot
	Issuer       PartySnapshot
	Letterhead   domain.Letterhead
	BankAccounts []domain.BankAccount
	Template     domain.DocumentTemplate
}

func RenderInvoicePreview(data PreviewData) (string, error) {
	computed, calcErr := domain.CalculateFinancialsWithCharges(data.Document.Items, data.Document.InvoiceDiscount, data.Document.TaxRateBPS, data.Document.AdditionalCharges)
	if calcErr != nil {
		return "", calcErr
	}
	data.Totals = computed
	if err := data.Template.Validate(); err != nil {
		return "", err
	}
	if data.Document.Number == "" {
		return "", fmt.Errorf("شماره سند برای پیش‌نمایش الزامی است")
	}
	if len(data.Document.Items) == 0 {
		return "", fmt.Errorf("سند بدون قلم قابل پیش‌نمایش نیست")
	}
	if strings.TrimSpace(data.Document.CustomerSnapshot) != "" {
		if err := json.Unmarshal([]byte(data.Document.CustomerSnapshot), &data.Customer); err != nil {
			return "", fmt.Errorf("اطلاعات snapshot مشتری نامعتبر است: %w", err)
		}
	}
	if strings.TrimSpace(data.Document.IssuerSnapshot) != "" {
		if err := json.Unmarshal([]byte(data.Document.IssuerSnapshot), &data.Issuer); err != nil {
			return "", fmt.Errorf("اطلاعات snapshot صادرکننده نامعتبر است: %w", err)
		}
	}
	var b strings.Builder
	orientation := "portrait"
	if data.Template.Orientation == "landscape" {
		orientation = "landscape"
	}
	fmt.Fprintf(&b, `<!doctype html><html lang="fa" dir="rtl"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>%s</title><style>
@page{size:%s %s;margin:14mm}*{box-sizing:border-box}body{direction:rtl;font-family:"%s",Tahoma,sans-serif;font-size:%dpt;color:#222;margin:0;padding:20px;background:#fff}header{border-bottom:3px solid %s;padding:0 0 14px;margin-bottom:18px;display:flex;align-items:center;gap:16px}.logo{max-width:120px;max-height:80px;object-fit:contain}.brand{flex:1}.brand h1{margin:0;color:%s;font-size:1.5em}.muted{color:#555;font-size:.9em}.meta,.parties{display:flex;justify-content:space-between;gap:18px;margin:14px 0}.parties section{width:50%%;border:1px solid #ddd;border-radius:6px;padding:10px}.parties h2{font-size:1em;margin:0 0 8px;color:%s}.items{width:100%%;border-collapse:collapse;margin-top:18px}.items th{background:%s;color:#fff}.items th,.items td{border:1px solid #ddd;padding:7px;text-align:right}.items td.num,.items th.num{text-align:left;white-space:nowrap}.totals{width:45%%;margin:18px 0 0 auto;border-collapse:collapse}.totals td{padding:6px;border-bottom:1px solid #ddd}.totals tr:last-child{font-weight:bold;color:%s;font-size:1.1em}.note{margin-top:18px;white-space:pre-wrap}.bank{margin-top:18px;padding:10px;border-top:1px solid #ddd}.bank h3{margin:0 0 6px;color:%s}.bank p{margin:3px 0}.footer{margin-top:28px;border-top:1px solid #ddd;padding-top:8px;color:#666;font-size:.85em}
</style></head><body>`,
		esc(data.Document.Number), esc(data.Template.PageSize), orientation, esc(data.Template.FontFamily), data.Template.FontSize, esc(data.Template.PrimaryColor), esc(data.Template.PrimaryColor), esc(data.Template.AccentColor), esc(data.Template.PrimaryColor), esc(data.Template.PrimaryColor), esc(data.Template.AccentColor))
	b.WriteString("<header>")
	if data.Template.ShowLogo && data.Template.LogoData != "" {
		fmt.Fprintf(&b, `<img class="logo" src="%s" alt="لوگو">`, esc(data.Template.LogoData))
	} else if data.Template.ShowLogo && data.Letterhead.LogoPath != "" && !strings.Contains(data.Letterhead.LogoPath, "://") {
		fmt.Fprintf(&b, `<img class="logo" src="file:///%s" alt="لوگو">`, esc(strings.ReplaceAll(data.Letterhead.LogoPath, `\`, "/")))
	}
	b.WriteString("<div class=\"brand\">")
	fmt.Fprintf(&b, "<h1>%s</h1>", esc(firstNonEmpty(data.Letterhead.Title, data.Issuer.Name, "فاکتور")))
	if data.Letterhead.Subtitle != "" {
		fmt.Fprintf(&b, "<div>%s</div>", esc(data.Letterhead.Subtitle))
	}
	if data.Letterhead.ContactLine != "" {
		fmt.Fprintf(&b, "<div class=\"muted\">%s</div>", esc(data.Letterhead.ContactLine))
	}
	b.WriteString("</div></header>")
	fmt.Fprintf(&b, `<div class="meta"><div>نوع فاکتور: <strong>%s</strong></div><div>شماره سند: <strong>%s</strong></div><div>تاریخ: %s</div><div>واحد پول: %s</div></div>`, esc(domain.DocumentTypeLabelFA(data.Document)), esc(data.Document.Number), esc(data.Document.IssueDate.Format("2006-01-02")), esc(string(data.Document.Currency)))
	b.WriteString(`<div class="parties"><section><h2>صادرکننده</h2>`)
	writeParty(&b, data.Issuer)
	b.WriteString(`</section><section><h2>مشتری</h2>`)
	writeParty(&b, data.Customer)
	b.WriteString("</section></div>")
	b.WriteString(`<table class="items"><thead><tr><th>ردیف</th><th>شرح</th><th>تعداد</th><th>واحد</th><th class="num">قیمت واحد</th><th class="num">تخفیف</th><th class="num">مبلغ</th></tr></thead><tbody>`)
	for i, item := range data.Document.Items {
		lineTotal, err := item.LineTotal()
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, `<tr><td>%d</td><td>%s</td><td>%d</td><td>%s</td><td class="num">%s</td><td class="num">%s</td><td class="num">%s</td></tr>`, i+1, esc(item.Description), item.Quantity, esc(item.Unit), money(item.UnitPrice), money(item.Discount), money(lineTotal))
	}
	fmt.Fprintf(&b, `<tr><td colspan="2"><strong>جمع تعداد اقلام</strong></td><td><strong>%d</strong></td><td colspan="4"></td></tr>`, data.Totals.TotalQuantity)
	b.WriteString("</tbody></table><table class=\"totals\">")
	writeTotal(&b, "جمع ناخالص", data.Totals.Subtotal, data.Document.Currency)
	if data.Totals.ItemDiscount > 0 {
		writeTotal(&b, "تخفیف اقلام", data.Totals.ItemDiscount, data.Document.Currency)
	}
	if data.Totals.InvoiceDiscount > 0 {
		writeTotal(&b, "تخفیف کلی", data.Totals.InvoiceDiscount, data.Document.Currency)
	}
	if data.Template.ShowTax {
		writeTotal(&b, "مالیات", data.Totals.Tax, data.Document.Currency)
	}
	writeTotal(&b, "جمع کل فاکتور", data.Totals.Total, data.Document.Currency)
	for _, charge := range data.Document.AdditionalCharges {
		writeTotal(&b, "هزینهٔ اضافه: "+charge.Description, charge.Amount, data.Document.Currency)
	}
	grand := data.Totals.GrandTotal
	if grand == 0 {
		grand = data.Totals.Total
	}
	if len(data.Document.AdditionalCharges) > 0 {
		writeTotal(&b, "مبلغ قابل پرداخت", grand, data.Document.Currency)
	}
	b.WriteString("</table>")
	fmt.Fprintf(&b, `<div class="note"><strong>به حروف:</strong> %s</div>`, esc(domain.AmountInWordsFA(grand, data.Document.Currency)))
	if data.Document.Note != "" {
		fmt.Fprintf(&b, `<div class="note"><strong>توضیحات:</strong> %s</div>`, esc(data.Document.Note))
	}
	selectedBanksRendered := false
	if data.Template.ShowBankAccounts && strings.TrimSpace(data.Document.BankAccountsSnapshot) != "" {
		var selected []domain.IssuerBankDetails
		if err := json.Unmarshal([]byte(data.Document.BankAccountsSnapshot), &selected); err != nil {
			return "", fmt.Errorf("snapshot حساب‌های بانکی نامعتبر است: %w", err)
		}
		if len(selected) > 0 {
			b.WriteString(`<div class="bank"><h3>اطلاعات پرداخت</h3>`)
			for _, a := range selected {
				fmt.Fprintf(&b, `<p>شماره حساب: <bdi dir="ltr">%s</bdi> | کارت: <bdi dir="ltr">%s</bdi> | شبا: <bdi dir="ltr">%s</bdi> | بانک: %s</p>`, esc(a.AccountNumber), esc(a.CardNumber), esc(banking.DisplayIBAN(a.IBAN)), esc(banking.ResolveBankName(a.BankName, a.IBAN, a.CardNumber)))
			}
			b.WriteString("</div>")
			selectedBanksRendered = true
		}
	}
	if !selectedBanksRendered && data.Template.ShowBankAccounts && len(data.BankAccounts) > 0 {
		b.WriteString(`<div class="bank"><h3>اطلاعات پرداخت</h3>`)
		for _, a := range data.BankAccounts {
			fmt.Fprintf(&b, `<p><strong>%s</strong> — صاحب حساب: %s</p><p>شماره حساب: <bdi dir="ltr">%s</bdi> | کارت: <bdi dir="ltr">%s</bdi> | شبا: <bdi dir="ltr">%s</bdi></p>`, esc(banking.ResolveBankName(a.BankName, a.IBAN, a.CardNumber)), esc(a.AccountHolder), esc(a.AccountNumber), esc(a.CardNumber), esc(banking.DisplayIBAN(a.IBAN)))
		}
		b.WriteString("</div>")
	}
	if data.Template.ShowSeal || data.Template.ShowSignature {
		b.WriteString(`<div style="display:flex;gap:12px;margin-top:22px;break-inside:avoid">`)
		if data.Template.ShowSeal {
			b.WriteString(`<div style="flex:1;min-height:90px;border:1px dashed #9aa6b2;border-radius:8px;display:flex;align-items:center;justify-content:center;padding:10px;color:#667085">`)
			if data.Template.SealData != "" {
				fmt.Fprintf(&b, `<div style="text-align:center"><img src="%s" alt="مهر" style="display:block;max-width:100%%;max-height:80px;object-fit:contain;margin:auto">مهر فروشگاه</div>`, esc(data.Template.SealData))
			} else {
				b.WriteString("مهر فروشگاه")
			}
			b.WriteString("</div>")
		}
		if data.Template.ShowSignature {
			b.WriteString(`<div style="flex:1;min-height:90px;border:1px dashed #9aa6b2;border-radius:8px;display:flex;align-items:center;justify-content:center;padding:10px;color:#667085">`)
			if data.Template.SignatureData != "" {
				fmt.Fprintf(&b, `<div style="text-align:center"><img src="%s" alt="امضا" style="display:block;max-width:100%%;max-height:80px;object-fit:contain;margin:auto">امضای فروشنده</div>`, esc(data.Template.SignatureData))
			} else {
				b.WriteString("امضای فروشنده")
			}
			b.WriteString("</div>")
		}
		b.WriteString("</div>")
	}
	if strings.TrimSpace(data.Template.ThankYou) != "" {
		fmt.Fprintf(&b, `<div style="margin-top:20px;padding-top:12px;border-top:1px solid #ddd;text-align:center;color:#555">%s</div>`, esc(data.Template.ThankYou))
	}
	b.WriteString(`<div class="footer">این پیش‌نمایش بر اساس اطلاعات ذخیره‌شده سند و قالب انتخابی تولید شده است.</div></body></html>`)
	return b.String(), nil
}
func writeParty(b *strings.Builder, p PartySnapshot) {
	if p.Name != "" {
		fmt.Fprintf(b, "<p><strong>%s</strong></p>", esc(p.Name))
	}
	if p.LegalName != "" {
		fmt.Fprintf(b, "<p>%s</p>", esc(p.LegalName))
	}
	if p.TaxID != "" {
		fmt.Fprintf(b, "<p>شناسه مالیاتی: %s</p>", esc(p.TaxID))
	}
	if p.Phone != "" {
		fmt.Fprintf(b, "<p>تلفن: %s</p>", esc(p.Phone))
	}
	if p.Email != "" {
		fmt.Fprintf(b, "<p>%s</p>", esc(p.Email))
	}
	if p.Address != "" {
		fmt.Fprintf(b, "<p>%s</p>", esc(p.Address))
	}
}
func writeTotal(b *strings.Builder, label string, amount domain.Money, c domain.Currency) {
	fmt.Fprintf(b, "<tr><td>%s</td><td class=\"num\">%s %s</td></tr>", esc(label), money(amount), esc(string(c)))
}
func money(m domain.Money) string { return fmt.Sprintf("%d", m) }
func esc(s string) string         { return html.EscapeString(s) }
func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
