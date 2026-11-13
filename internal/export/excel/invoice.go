package excel

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
	"github.com/xuri/excelize/v2"
)

// ExportInvoice writes a printable, Persian-first invoice sheet. Monetary cells
// remain numeric so Excel formulas, sorting, and accounting workflows keep working.
func ExportInvoice(data invoicehtml.PreviewData, outputPath string) error {
	computed, calcErr := domain.CalculateFinancialsWithCharges(data.Document.Items, data.Document.InvoiceDiscount, data.Document.TaxRateBPS, data.Document.AdditionalCharges)
	if calcErr != nil {
		return calcErr
	}
	data.Totals = computed
	if strings.TrimSpace(outputPath) == "" {
		return errors.New("مسیر خروجی Excel الزامی است")
	}
	if err := data.Template.Validate(); err != nil {
		return err
	}
	if len(data.Document.Items) == 0 {
		return errors.New("سند بدون قلم قابل خروجی نیست")
	}
	if strings.ToLower(filepath.Ext(outputPath)) != ".xlsx" {
		return errors.New("پسوند فایل خروجی باید .xlsx باشد")
	}
	if raw := strings.TrimSpace(data.Document.CustomerSnapshot); raw != "" {
		if err := json.Unmarshal([]byte(raw), &data.Customer); err != nil {
			return fmt.Errorf("snapshot مشتری نامعتبر است: %w", err)
		}
	}
	if raw := strings.TrimSpace(data.Document.IssuerSnapshot); raw != "" {
		if err := json.Unmarshal([]byte(raw), &data.Issuer); err != nil {
			return fmt.Errorf("snapshot صادرکننده نامعتبر است: %w", err)
		}
	}
	if raw := strings.TrimSpace(data.Document.BankAccountsSnapshot); raw != "" {
		var selected []domain.IssuerBankDetails
		if err := json.Unmarshal([]byte(raw), &selected); err != nil {
			return fmt.Errorf("snapshot حساب‌های بانکی نامعتبر است: %w", err)
		}
		data.BankAccounts = make([]domain.BankAccount, 0, len(selected))
		for _, a := range selected {
			data.BankAccounts = append(data.BankAccounts, domain.BankAccount{BankName: banking.ResolveBankName(a.BankName, a.IBAN, a.CardNumber), AccountNumber: a.AccountNumber, CardNumber: a.CardNumber, IBAN: a.IBAN})
		}
	}

	book := excelize.NewFile()
	defer book.Close()
	const sheet = "فاکتور"
	if err := book.SetSheetName("Sheet1", sheet); err != nil {
		return err
	}

	// Persian-first sheet: column A appears on the right and cell text follows RTL.
	rightToLeft, showGridLines, zoom := true, false, 90.0
	if err := book.SetSheetView(sheet, 0, &excelize.ViewOptions{
		RightToLeft:   &rightToLeft,
		ShowGridLines: &showGridLines,
		ZoomScale:     &zoom,
	}); err != nil {
		return err
	}

	navy := strings.TrimPrefix(data.Template.PrimaryColor, "#")
	if len(navy) != 6 {
		navy = "285082"
	}
	accent := strings.TrimPrefix(data.Template.AccentColor, "#")
	if len(accent) != 6 {
		accent = "4777A8"
	}
	light := "EEF3F8"
	border := "D8E1EB"
	white := "FFFFFF"
	dark := "1D2939"
	style := func(s *excelize.Style) (int, error) { return book.NewStyle(s) }
	brandStyle, err := style(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 16, Color: navy}, Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "center", WrapText: true}, Border: []excelize.Border{{Type: "left", Style: 1, Color: border}, {Type: "right", Style: 1, Color: border}, {Type: "top", Style: 1, Color: border}}})
	if err != nil {
		return err
	}
	titleStyle, err := style(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 20, Color: navy}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"DDEBFC"}}, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"}, Border: []excelize.Border{{Type: "top", Style: 1, Color: border}, {Type: "bottom", Style: 1, Color: border}}})
	if err != nil {
		return err
	}
	codeStyle, err := style(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 11, Color: navy}, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true}, Border: []excelize.Border{{Type: "right", Style: 1, Color: border}, {Type: "top", Style: 1, Color: border}}})
	if err != nil {
		return err
	}
	subtitleStyle, err := style(&excelize.Style{Font: &excelize.Font{Size: 10, Color: "667085"}, Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "center", WrapText: true}})
	if err != nil {
		return err
	}
	sectionStyle, err := style(&excelize.Style{Font: &excelize.Font{Bold: true, Color: white, Size: 11}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{accent}}, Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "center"}, Border: []excelize.Border{{Type: "bottom", Style: 1, Color: border}}})
	if err != nil {
		return err
	}
	labelStyle, err := style(&excelize.Style{Font: &excelize.Font{Bold: true, Color: navy}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{light}}, Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "center", WrapText: true}, Border: []excelize.Border{{Type: "bottom", Style: 1, Color: border}}})
	if err != nil {
		return err
	}
	textStyle, err := style(&excelize.Style{Font: &excelize.Font{Color: dark, Size: 10}, Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "center", WrapText: true}, Border: []excelize.Border{{Type: "bottom", Style: 1, Color: border}}})
	if err != nil {
		return err
	}
	alternateStyle, err := style(&excelize.Style{Font: &excelize.Font{Color: dark, Size: 10}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"F7F9FC"}}, Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "center", WrapText: true}, Border: []excelize.Border{{Type: "bottom", Style: 1, Color: border}}})
	if err != nil {
		return err
	}
	headerStyle, err := style(&excelize.Style{Font: &excelize.Font{Bold: true, Color: white}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{navy}}, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true}, Border: []excelize.Border{{Type: "bottom", Style: 2, Color: accent}}})
	if err != nil {
		return err
	}
	// Built-in format 3 is the locale-neutral integer format #,##0. It keeps
	// amount cells numeric while ensuring thousands separators are displayed.
	moneyStyle, err := style(&excelize.Style{NumFmt: 3, Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "center"}, Border: []excelize.Border{{Type: "bottom", Style: 1, Color: border}}})
	if err != nil {
		return err
	}
	totalStyle, err := style(&excelize.Style{Font: &excelize.Font{Bold: true, Color: navy, Size: 11}, NumFmt: 3, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{light}}, Border: []excelize.Border{{Type: "top", Style: 1, Color: accent}, {Type: "bottom", Style: 1, Color: border}}, Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "center"}})
	if err != nil {
		return err
	}
	finalStyle, err := style(&excelize.Style{Font: &excelize.Font{Bold: true, Color: white, Size: 13}, NumFmt: 3, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{navy}}, Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "center"}, Border: []excelize.Border{{Type: "top", Style: 2, Color: accent}}})
	if err != nil {
		return err
	}

	merge := func(a, b string) error { return book.MergeCell(sheet, a, b) }
	set := func(cell string, value interface{}) error { return book.SetCellValue(sheet, cell, value) }
	cell := func(col, row int) string { n, _ := excelize.CoordinatesToCellName(col, row); return n }
	issuerName := first(data.Issuer.Name, data.Issuer.LegalName)
	customerName := first(data.Customer.Name, data.Customer.LegalName)
	title := first(data.Letterhead.Title, issuerName, "فاکتور")
	for _, pair := range [][2]string{{"A1", "C1"}, {"A2", "C2"}, {"D1", "F2"}, {"G1", "G2"}, {"A5", "C5"}, {"D5", "G5"}, {"A6", "C6"}, {"D6", "G6"}, {"A7", "C7"}, {"D7", "G7"}, {"A8", "C8"}, {"D8", "G8"}, {"A10", "D10"}, {"E10", "G10"}, {"A11", "D11"}, {"E11", "G11"}, {"A12", "D12"}, {"E12", "G12"}, {"A13", "D13"}, {"E13", "G13"}} {
		if err := merge(pair[0], pair[1]); err != nil {
			return err
		}
	}
	if err := set("A1", title); err != nil {
		return err
	}
	if err := set("A2", first(data.Letterhead.Subtitle, data.Letterhead.ContactLine, "فروش و خدمات حرفه ای")); err != nil {
		return err
	}
	if err := set("D1", domain.DocumentTypeLabelFA(data.Document)); err != nil {
		return err
	}
	if err := set("G1", "شماره\n"+data.Document.Number); err != nil {
		return err
	}
	if err := book.SetCellStyle(sheet, "A1", "C2", brandStyle); err != nil {
		return err
	}
	if err := book.SetCellStyle(sheet, "D1", "F2", titleStyle); err != nil {
		return err
	}
	if err := book.SetCellStyle(sheet, "G1", "G2", codeStyle); err != nil {
		return err
	}
	if err := book.SetRowHeight(sheet, 1, 30); err != nil {
		return err
	}
	if err := book.SetRowHeight(sheet, 2, 22); err != nil {
		return err
	}

	// Document metadata, followed by two clearly separated party blocks.
	metadata := []struct {
		r            int
		label, value string
	}{{5, "شماره سند", data.Document.Number}, {6, "تاریخ صدور", jalaliDate(data.Document.IssueDate)}, {7, "نوع فاکتور", domain.DocumentTypeLabelFA(data.Document)}, {8, "واحد پول", currencyName(data.Document.Currency)}}
	for _, m := range metadata {
		if err := set(fmt.Sprintf("A%d", m.r), m.label); err != nil {
			return err
		}
		if err := set(fmt.Sprintf("D%d", m.r), m.value); err != nil {
			return err
		}
		if err := book.SetCellStyle(sheet, fmt.Sprintf("A%d", m.r), fmt.Sprintf("C%d", m.r), labelStyle); err != nil {
			return err
		}
		if err := book.SetCellStyle(sheet, fmt.Sprintf("D%d", m.r), fmt.Sprintf("G%d", m.r), textStyle); err != nil {
			return err
		}
	}
	if err := set("A10", "مشخصات صادرکننده"); err != nil {
		return err
	}
	if err := set("E10", "مشخصات مشتری"); err != nil {
		return err
	}
	if err := book.SetCellStyle(sheet, "A10", "D10", sectionStyle); err != nil {
		return err
	}
	if err := book.SetCellStyle(sheet, "E10", "G10", sectionStyle); err != nil {
		return err
	}
	issuerLines := []string{issuerName, data.Issuer.Phone, data.Issuer.Email, data.Issuer.Address}
	customerLines := []string{customerName, data.Customer.Phone, data.Customer.Email, data.Customer.Address}
	for i := 0; i < 4; i++ {
		r := 11 + i
		if err := set(fmt.Sprintf("A%d", r), issuerLines[i]); err != nil {
			return err
		}
		if err := set(fmt.Sprintf("E%d", r), customerLines[i]); err != nil {
			return err
		}
		if err := book.SetCellStyle(sheet, fmt.Sprintf("A%d", r), fmt.Sprintf("D%d", r), textStyle); err != nil {
			return err
		}
		if err := book.SetCellStyle(sheet, fmt.Sprintf("E%d", r), fmt.Sprintf("G%d", r), textStyle); err != nil {
			return err
		}
		if err := book.SetRowHeight(sheet, r, 22); err != nil {
			return err
		}
	}

	// Items table is spacious, with wrapped descriptions and numeric amount cells.
	headerRow := 16
	headers := []string{"ردیف", "شرح کالا / خدمات", "تعداد", "واحد", "قیمت واحد", "تخفیف", "مبلغ"}
	for i, h := range headers {
		if err := set(cell(i+1, headerRow), h); err != nil {
			return err
		}
	}
	if err := book.SetCellStyle(sheet, "A16", "G16", headerStyle); err != nil {
		return err
	}
	if err := book.SetRowHeight(sheet, headerRow, 28); err != nil {
		return err
	}
	for i, item := range data.Document.Items {
		r := headerRow + 1 + i
		line, err := item.LineTotal()
		if err != nil {
			return err
		}
		values := []interface{}{i + 1, item.Description, item.Quantity, item.Unit, int64(item.UnitPrice), int64(item.Discount), int64(line)}
		for j, v := range values {
			if err := set(cell(j+1, r), v); err != nil {
				return err
			}
		}
		rowStyle := textStyle
		if i%2 == 1 {
			rowStyle = alternateStyle
		}
		if err := book.SetCellStyle(sheet, fmt.Sprintf("A%d", r), fmt.Sprintf("G%d", r), rowStyle); err != nil {
			return err
		}
		if err := book.SetCellStyle(sheet, fmt.Sprintf("E%d", r), fmt.Sprintf("G%d", r), moneyStyle); err != nil {
			return err
		}
		if err := book.SetCellStyle(sheet, fmt.Sprintf("A%d", r), fmt.Sprintf("A%d", r), headerStyle); err != nil {
			return err
		}
		if err := book.SetRowHeight(sheet, r, 30); err != nil {
			return err
		}
	}
	r := headerRow + len(data.Document.Items) + 2
	writeTotal := func(label string, value domain.Money, final bool) error {
		if err := merge(fmt.Sprintf("A%d", r), fmt.Sprintf("F%d", r)); err != nil {
			return err
		}
		if err := set(fmt.Sprintf("A%d", r), label); err != nil {
			return err
		}
		if err := set(fmt.Sprintf("G%d", r), int64(value)); err != nil {
			return err
		}
		st := totalStyle
		if final {
			st = finalStyle
		}
		if err := book.SetCellStyle(sheet, fmt.Sprintf("A%d", r), fmt.Sprintf("G%d", r), st); err != nil {
			return err
		}
		if err := book.SetRowHeight(sheet, r, 27); err != nil {
			return err
		}
		r++
		return nil
	}
	if err := writeTotal("جمع تعداد اقلام", domain.Money(data.Totals.TotalQuantity), false); err != nil {
		return err
	}
	if err := writeTotal("جمع اقلام", data.Totals.Subtotal, false); err != nil {
		return err
	}
	if data.Totals.ItemDiscount > 0 {
		if err := writeTotal("تخفیف اقلام", data.Totals.ItemDiscount, false); err != nil {
			return err
		}
	}
	if data.Totals.InvoiceDiscount > 0 {
		if err := writeTotal("تخفیف کلی", data.Totals.InvoiceDiscount, false); err != nil {
			return err
		}
	}
	if data.Template.ShowTax {
		if err := writeTotal("مالیات", data.Totals.Tax, false); err != nil {
			return err
		}
	}
	if err := writeTotal("مبلغ نهایی", data.Totals.Total, true); err != nil {
		return err
	}
	for _, charge := range data.Document.AdditionalCharges {
		if err := writeTotal("هزینهٔ اضافه: "+charge.Description, charge.Amount, false); err != nil {
			return err
		}
	}
	grand := data.Totals.GrandTotal
	if grand == 0 {
		grand = data.Totals.Total
	}
	if len(data.Document.AdditionalCharges) > 0 {
		if err := writeTotal("مبلغ قابل پرداخت", grand, true); err != nil {
			return err
		}
	}
	r++
	if err := merge(fmt.Sprintf("A%d", r), fmt.Sprintf("G%d", r)); err != nil {
		return err
	}
	if err := set(fmt.Sprintf("A%d", r), "به حروف: "+domain.AmountInWordsFA(grand, data.Document.Currency)); err != nil {
		return err
	}
	r++

	if strings.TrimSpace(data.Document.Note) != "" {
		r++
		if err := merge(fmt.Sprintf("A%d", r), fmt.Sprintf("G%d", r)); err != nil {
			return err
		}
		if err := set(fmt.Sprintf("A%d", r), "توضیحات و شرایط"); err != nil {
			return err
		}
		if err := book.SetCellStyle(sheet, fmt.Sprintf("A%d", r), fmt.Sprintf("G%d", r), sectionStyle); err != nil {
			return err
		}
		r++
		if err := merge(fmt.Sprintf("A%d", r), fmt.Sprintf("G%d", r)); err != nil {
			return err
		}
		if err := set(fmt.Sprintf("A%d", r), data.Document.Note); err != nil {
			return err
		}
		if err := book.SetCellStyle(sheet, fmt.Sprintf("A%d", r), fmt.Sprintf("G%d", r), textStyle); err != nil {
			return err
		}
		if err := book.SetRowHeight(sheet, r, 38); err != nil {
			return err
		}
		r++
	}
	if data.Template.ShowBankAccounts && len(data.BankAccounts) > 0 {
		r++
		if err := merge(fmt.Sprintf("A%d", r), fmt.Sprintf("G%d", r)); err != nil {
			return err
		}
		if err := set(fmt.Sprintf("A%d", r), "اطلاعات پرداخت"); err != nil {
			return err
		}
		if err := book.SetCellStyle(sheet, fmt.Sprintf("A%d", r), fmt.Sprintf("G%d", r), sectionStyle); err != nil {
			return err
		}
		r++
		for _, a := range data.BankAccounts {
			parts := make([]string, 0, 5)
			if strings.TrimSpace(a.AccountHolder) != "" {
				parts = append(parts, "صاحب حساب: "+strings.TrimSpace(a.AccountHolder))
			}
			if strings.TrimSpace(a.AccountNumber) != "" {
				parts = append(parts, "شماره حساب: "+strings.TrimSpace(a.AccountNumber))
			}
			if strings.TrimSpace(a.CardNumber) != "" {
				parts = append(parts, "کارت: "+strings.TrimSpace(a.CardNumber))
			}
			if strings.TrimSpace(a.IBAN) != "" {
				parts = append(parts, "شبا: "+banking.DisplayIBAN(strings.TrimSpace(a.IBAN)))
			}
			if strings.TrimSpace(a.BankName) != "" {
				parts = append(parts, "بانک: "+strings.TrimSpace(a.BankName))
			}
			if err := merge(fmt.Sprintf("A%d", r), fmt.Sprintf("G%d", r)); err != nil {
				return err
			}
			if err := set(fmt.Sprintf("A%d", r), strings.Join(parts, "  |  ")); err != nil {
				return err
			}
			if err := book.SetCellStyle(sheet, fmt.Sprintf("A%d", r), fmt.Sprintf("G%d", r), textStyle); err != nil {
				return err
			}
			r++
		}
	}
	r++
	if err := merge(fmt.Sprintf("A%d", r), fmt.Sprintf("G%d", r)); err != nil {
		return err
	}
	if err := set(fmt.Sprintf("A%d", r), "از اعتماد شما سپاسگزاریم"); err != nil {
		return err
	}
	if err := book.SetCellStyle(sheet, fmt.Sprintf("A%d", r), fmt.Sprintf("G%d", r), subtitleStyle); err != nil {
		return err
	}

	for col, width := range map[string]float64{"A": 11, "B": 38, "C": 12, "D": 13, "E": 19, "F": 18, "G": 21} {
		if err := book.SetColWidth(sheet, col, col, width); err != nil {
			return err
		}
	}
	// Freeze the column labels while scrolling through long invoices.
	if err := book.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: headerRow, TopLeftCell: "A17", ActivePane: "bottomLeft", Selection: []excelize.Selection{{SQRef: "A17", ActiveCell: "A17", Pane: "bottomLeft"}}}); err != nil {
		return err
	}
	size, fitWidth, fitHeight, orientation := 9, 1, 0, "portrait"
	if data.Template.Orientation == "landscape" {
		orientation = "landscape"
	}
	if err := book.SetPageLayout(sheet, &excelize.PageLayoutOptions{Orientation: &orientation, Size: &size, FitToWidth: &fitWidth, FitToHeight: &fitHeight}); err != nil {
		return err
	}
	left, right, top, bottom, header, footer := 0.3, 0.3, 0.45, 0.45, 0.2, 0.2
	if err := book.SetPageMargins(sheet, &excelize.PageLayoutMarginsOptions{Left: &left, Right: &right, Top: &top, Bottom: &bottom, Header: &header, Footer: &footer}); err != nil {
		return err
	}
	if err := book.SetHeaderFooter(sheet, &excelize.HeaderFooterOptions{OddFooter: "&R" + documentLabel(data.Document.Kind) + " " + data.Document.Number + "&Cصفحه &P از &N"}); err != nil {
		return err
	}
	if err := book.SetSheetProps(sheet, &excelize.SheetPropsOptions{FitToPage: boolPtr(true)}); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}
	if err := book.SaveAs(outputPath); err != nil {
		return fmt.Errorf("ذخیره فایل XLSX ناموفق بود: %w", err)
	}
	return nil
}

func boolPtr(v bool) *bool { return &v }
func jalaliDate(t time.Time) string {
	d, err := domain.GregorianToJalali(t.Year(), int(t.Month()), t.Day())
	if err != nil {
		return t.Format("2006/01/02")
	}
	return fmt.Sprintf("%04d/%02d/%02d", d.Year, d.Month, d.Day)
}
func currencyName(c domain.Currency) string {
	if c == domain.CurrencyRial {
		return "ریال"
	}
	return "تومان"
}
func documentKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "proforma", "pro_forma", "estimate":
		return "پیش‌فاکتور"
	default:
		return "فاکتور"
	}
}
func documentTitle(kind string) string {
	if strings.EqualFold(strings.TrimSpace(kind), "proforma") {
		return "پیش فاکتور فروش"
	}
	return "فاکتور فروش"
}
func documentLabel(kind string) string {
	if strings.EqualFold(strings.TrimSpace(kind), "proforma") {
		return "پیش فاکتور"
	}
	return "فاکتور"
}
func first(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return "فاکتور"
}
