package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hesora-ir/hesora-free/internal/application"
	"github.com/hesora-ir/hesora-free/internal/domain"
	invoiceexcel "github.com/hesora-ir/hesora-free/internal/export/excel"
	invoicepdf "github.com/hesora-ir/hesora-free/internal/export/pdf"
	invoicehtml "github.com/hesora-ir/hesora-free/internal/render/html"
	qt "github.com/mappu/miqt/qt6"
)

type invoicePartySnapshot struct {
	Name      string `json:"name"`
	LegalName string `json:"legal_name"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	Address   string `json:"address"`
	TaxID     string `json:"tax_id"`
}

type invoiceItemEditor struct {
	row      *qt.QWidget
	name     *qt.QLineEdit
	quantity *qt.QLineEdit
	unit     *qt.QLineEdit
	price    *qt.QLineEdit
	remove   *qt.QPushButton
	total    *qt.QLabel
	discount domain.Money
}

func buildInvoicesPage(parent *qt.QWidget, contacts *application.Contacts, invoices *application.Invoices, templates *application.Templates, ctx context.Context) page {
	p := newPage(parent, "فاکتورها", "مدیریت یکپارچه پیش‌فاکتورها، مطالبات و اسناد تسویه‌شده")
	p.widget.SetStyleSheet(p.widget.StyleSheet() + `
QWidget#invoiceCard { background: white; border: 1px solid #dfe7f1; border-radius: 12px; }
QLabel#invoiceCardTitle { color: #17345f; font-size: 12pt; font-weight: 700; }
QLabel#invoiceMuted { color: #718198; }
QLabel#invoiceAmount { color: #17345f; font-size: 14pt; font-weight: 700; }
QLabel#invoiceStatus { padding: 5px 9px; border-radius: 8px; font-weight: 700; background: #edf2f8; }
QPushButton#secondaryAction { background: #edf3fb; color: #23456f; border: 1px solid #d8e3f1; }
`)
	search := qt.NewQLineEdit(p.widget)
	search.SetPlaceholderText("جست‌وجو بر اساس شماره فاکتور یا نام مشتری")
	refreshButton := qt.NewQPushButton5("به‌روزرسانی", p.widget)
	paymentHistoryButton := qt.NewQPushButton5("دفتر دریافت‌ها", p.widget)
	searchRow := qt.NewQHBoxLayout2()
	searchRow.AddWidget(search.QWidget)
	searchRow.AddWidget(refreshButton.QWidget)
	searchRow.AddWidget(paymentHistoryButton.QWidget)
	p.layout.AddLayout(searchRow.QLayout)
	paymentHistoryButton.OnClicked(func() { showInvoicePaymentHistoryWindow(parent, invoices, ctx) })

	exportHint := qt.NewQLabel5("خروجی هر سند از کارت همان سند در دسترس خواهد بود.", p.widget)
	exportHint.SetWordWrap(true)
	setInvoiceObjectName(exportHint, "invoiceMuted")
	p.layout.AddWidget(exportHint.QWidget)

	summaryRow := qt.NewQHBoxLayout2()
	p.layout.AddLayout(summaryRow.QLayout)
	categoryTabs := qt.NewQTabWidget(p.widget)
	p.layout.AddWidget(categoryTabs.QWidget)

	type invoiceCategory struct {
		title  string
		filter func(domain.InvoiceListItem) bool
	}
	categories := []invoiceCategory{
		{title: "پرداخت‌نشده", filter: func(item domain.InvoiceListItem) bool {
			return item.Kind == "invoice" && item.Status == "issued" && item.Balance > 0
		}},
		{title: "پیش‌فاکتورها", filter: func(item domain.InvoiceListItem) bool {
			return item.Kind == "proforma"
		}},
		{title: "پرداخت‌شده", filter: func(item domain.InvoiceListItem) bool {
			return item.Kind == "invoice" && item.Status == "issued" && item.Balance == 0
		}},
	}
	type categoryView struct {
		scroll    *qt.QScrollArea
		container *qt.QWidget
		layout    *qt.QVBoxLayout
		count     *qt.QLabel
	}
	views := make([]categoryView, 0, len(categories))
	for _, category := range categories {
		container := qt.NewQWidget(categoryTabs.QWidget)
		layout := qt.NewQVBoxLayout2()
		layout.SetContentsMargins(12, 12, 12, 12)
		layout.SetSpacing(12)
		container.SetLayout(layout.QLayout)
		count := qt.NewQLabel5("۰ سند", container)
		setInvoiceObjectName(count, "invoiceMuted")
		layout.AddWidget(count.QWidget)
		layout.AddStretch()
		scroll := qt.NewQScrollArea(categoryTabs.QWidget)
		scroll.SetWidgetResizable(true)
		scroll.SetWidget(container)
		categoryTabs.AddTab(scroll.QWidget, category.title)
		views = append(views, categoryView{scroll: scroll, container: container, layout: layout, count: count})
	}

	status := qt.NewQLabel5("", p.widget)
	status.SetWordWrap(true)
	p.layout.AddWidget(status.QWidget)
	oldCards := make([][]*qt.QWidget, len(categories))
	var refresh func()
	refresh = func() {
		for i, cards := range oldCards {
			for _, card := range cards {
				card.SetVisible(false)
			}
			oldCards[i] = nil
		}
		items, err := invoices.List(ctx, strings.TrimSpace(search.Text()))
		if err != nil {
			status.SetText("خواندن اسناد ناموفق بود: " + err.Error())
			return
		}
		counts := make([]int, len(categories))
		for _, item := range items {
			for categoryIndex, category := range categories {
				if !category.filter(item) {
					continue
				}
				counts[categoryIndex]++
				record := item
				view := views[categoryIndex]
				card := qt.NewQWidget(view.container)
				setInvoiceObjectName(card, "invoiceCard")
				card.SetStyleSheet("QWidget#invoiceCard { background: white; border: 1px solid #dfe7f1; border-radius: 12px; }")
				cardLayout := qt.NewQVBoxLayout2()
				cardLayout.SetContentsMargins(16, 14, 16, 14)
				cardLayout.SetSpacing(8)
				card.SetLayout(cardLayout.QLayout)

				titleRow := qt.NewQHBoxLayout2()
				title := domain.DocumentTypeNameFA(record.DocumentType, record.Kind)
				titleLabel := qt.NewQLabel5(title+"  #"+record.Number, card)
				setInvoiceObjectName(titleLabel, "invoiceCardTitle")
				titleLabel.SetWordWrap(true)
				titleRow.AddWidget(titleLabel.QWidget)
				titleRow.AddStretch()
				statusText := "پیش‌نویس"
				statusStyle := "color:#8a5a00;background:#fff4d6;"
				if record.Status == "issued" {
					statusText = "صادرشده"
					statusStyle = "color:#155eef;background:#eaf1ff;"
				}
				if categoryIndex == 0 {
					statusText = "مانده‌دار"
					statusStyle = "color:#a33a24;background:#fff0ed;"
				}
				if categoryIndex == 2 {
					statusText = "تسویه‌شده"
					statusStyle = "color:#18794e;background:#e9f8ef;"
				}
				statusLabel := qt.NewQLabel5(statusText, card)
				setInvoiceObjectName(statusLabel, "invoiceStatus")
				statusLabel.SetStyleSheet("QLabel#invoiceStatus { padding:5px 9px; border-radius:8px; font-weight:700; " + statusStyle + " }")
				titleRow.AddWidget(statusLabel.QWidget)
				cardLayout.AddLayout(titleRow.QLayout)

				customerLabel := qt.NewQLabel5("مشتری: "+record.Customer, card)
				setInvoiceObjectName(customerLabel, "invoiceMuted")
				customerLabel.SetWordWrap(true)
				cardLayout.AddWidget(customerLabel.QWidget)
				currency := "تومان"
				if record.Currency == "IRR" {
					currency = "ریال"
				}
				amountRow := qt.NewQHBoxLayout2()
				totalLabel := qt.NewQLabel5("مبلغ کل: "+formatCustomerMoney(record.Total)+" "+currency, card)
				setInvoiceObjectName(totalLabel, "invoiceAmount")
				totalLabel.SetWordWrap(true)
				amountRow.AddWidget(totalLabel.QWidget)
				amountRow.AddStretch()
				balanceLabel := qt.NewQLabel5("مانده: "+formatCustomerMoney(record.Balance)+" "+currency, card)
				setInvoiceObjectName(balanceLabel, "invoiceMuted")
				amountRow.AddWidget(balanceLabel.QWidget)
				cardLayout.AddLayout(amountRow.QLayout)

				actions := qt.NewQHBoxLayout2()
				if record.Kind == "proforma" && record.Status == "draft" && record.DocumentType != "quotation" {
					convert := qt.NewQPushButton5("تبدیل به فاکتور", card)
					convert.OnClicked(func() {
						if err := invoices.ConvertProforma(ctx, record.ID); err != nil {
							status.SetText("تبدیل ناموفق بود: " + err.Error())
						} else {
							status.SetText("پیش‌فاکتور تبدیل شد و بدهی مشتری ثبت شد.")
						}
						refresh()
					})
					actions.AddWidget(convert.QWidget)
				}
				if record.Status == "draft" || (record.Kind == "invoice" && record.Status == "issued" && record.Balance > 0) {
					edit := qt.NewQPushButton5("ویرایش", card)
					setInvoiceObjectName(edit, "secondaryAction")
					edit.OnClicked(func() {
						doc, err := invoices.Get(ctx, record.ID)
						if err != nil {
							status.SetText("بازکردن سند ناموفق بود: " + err.Error())
							return
						}
						window := qt.NewQMainWindow2()
						window.QWidget.SetWindowTitle("ویرایش سند " + record.Number)
						window.QWidget.SetMinimumSize2(960, 700)
						editor := buildInvoicePage(window.QWidget, contacts, invoices, ctx, &doc, invoiceEditorOptions{templates: templates, contacts: contacts})
						window.SetCentralWidget(editor.widget)
						window.Show()
					})
					actions.AddWidget(edit.QWidget)
				}
				if record.Kind == "invoice" && record.Status == "issued" && record.Balance > 0 {
					pay := qt.NewQPushButton5("ثبت پرداخت", card)
					setInvoiceObjectName(pay, "secondaryAction")
					pay.OnClicked(func() {
						showInvoicePaymentWindow(parent, record, invoices, ctx, func() {
							refresh()
							status.SetText("فهرست پس از ثبت پرداخت به‌روزرسانی شد.")
						})
					})
					actions.AddWidget(pay.QWidget)
				}
				pdfButton := qt.NewQPushButton5("خروجی PDF", card)
				setInvoiceObjectName(pdfButton, "secondaryAction")
				pdfButton.SetToolTip("ذخیره نسخه PDF سند")
				pdfButton.OnClicked(func() {
					path := qt.QFileDialog_GetSaveFileName4(parent, "ذخیره فاکتور به PDF", record.Number+".pdf", "PDF (*.pdf)")
					if path == "" {
						return
					}
					if strings.ToLower(filepath.Ext(path)) != ".pdf" {
						path += ".pdf"
					}
					fontDirectory, fontName, fontErr := findPDFFont()
					if fontErr != nil {
						qt.QMessageBox_Critical(parent, "فونت PDF در دسترس نیست", fontErr.Error())
						return
					}
					if err := exportInvoiceFile(ctx, record.ID, path, "pdf", contacts, invoices, templates, fontDirectory, fontName); err != nil {
						qt.QMessageBox_Critical(parent, "خروجی PDF ناموفق بود", err.Error())
						return
					}
					qt.QMessageBox_Information(parent, "خروجی آماده شد", "فایل PDF ذخیره شد:\n"+path)
				})
				excelButton := qt.NewQPushButton5("خروجی Excel", card)
				setInvoiceObjectName(excelButton, "secondaryAction")
				excelButton.SetToolTip("ذخیره نسخه قابل ویرایش XLSX سند")
				excelButton.OnClicked(func() {
					path := qt.QFileDialog_GetSaveFileName4(parent, "ذخیره فاکتور به Excel", record.Number+".xlsx", "Excel Workbook (*.xlsx)")
					if path == "" {
						return
					}
					if strings.ToLower(filepath.Ext(path)) != ".xlsx" {
						path += ".xlsx"
					}
					if err := exportInvoiceFile(ctx, record.ID, path, "xlsx", contacts, invoices, templates, ""); err != nil {
						qt.QMessageBox_Critical(parent, "خروجی Excel ناموفق بود", err.Error())
						return
					}
					qt.QMessageBox_Information(parent, "خروجی آماده شد", "فایل Excel ذخیره شد:\n"+path)
				})
				actions.AddWidget(pdfButton.QWidget)
				printButton := qt.NewQPushButton5("چاپ", card)
				setInvoiceObjectName(printButton, "secondaryAction")
				printButton.SetToolTip("بازکردن نسخهٔ آمادهٔ چاپ فاکتور")
				printButton.OnClicked(func() {
					path, err := cachedInvoicePDF(ctx, record.ID, contacts, invoices, templates)
					if err != nil {
						qt.QMessageBox_Critical(parent, "آماده‌سازی چاپ ناموفق بود", err.Error())
						return
					}
					if !openPDFForPrinting(path) {
						qt.QMessageBox_Critical(parent, "بازکردن PDF ناموفق بود", "فایل آماده شد، اما نمایشگر PDF باز نشد.")
						return
					}
				})
				actions.AddWidget(printButton.QWidget)
				actions.AddWidget(excelButton.QWidget)
				cardLayout.AddLayout(actions.QLayout)
				view.layout.AddWidget(card)
				oldCards[categoryIndex] = append(oldCards[categoryIndex], card)
			}
		}
		for i, view := range views {
			view.count.SetText(fmt.Sprintf("%d سند", counts[i]))

		}
		status.SetText(fmt.Sprintf("در مجموع %d سند پیدا شد.", len(items)))
	}
	refreshButton.OnClicked(refresh)
	search.OnTextChanged(func(string) { refresh() })
	refresh()
	scroll := qt.NewQScrollArea(parent)
	scroll.SetWidgetResizable(true)
	scroll.SetWidget(p.widget)
	return page{widget: scroll.QWidget, layout: p.layout}
}

// exportInvoiceFile builds one shared PreviewData model for PDF and XLSX so both
// exporters consume the same stored document and financial totals.
func exportInvoiceFile(ctx context.Context, invoiceID int64, outputPath, format string, contacts *application.Contacts, invoices *application.Invoices, templates *application.Templates, pdfFontDirectory string, pdfFontName ...string) error {
	doc, err := invoices.Get(ctx, invoiceID)
	if err != nil {
		return fmt.Errorf("خواندن سند برای خروجی: %w", err)
	}
	totals, err := domain.CalculateFinancialsWithCharges(doc.Items, doc.InvoiceDiscount, doc.TaxRateBPS, doc.AdditionalCharges)
	if err != nil {
		return fmt.Errorf("محاسبه مبلغ سند: %w", err)
	}

	template := domain.DocumentTemplate{
		Name: "قالب پیش‌فرض", PageSize: "A4", Orientation: "portrait",
		FontFamily: "IRANSansX", FontSize: 10, PrimaryColor: "#285082",
		AccentColor: "#4777A8", ShowLogo: true, ShowTax: true, ShowBankAccounts: true,
	}
	if templates != nil {
		savedTemplates, err := templates.DocumentTemplates(ctx)
		if err != nil {
			return fmt.Errorf("خواندن قالب‌های خروجی: %w", err)
		}
		if len(savedTemplates) > 0 {
			template = savedTemplates[0]
		}
	}
	if err := template.Validate(); err != nil {
		return fmt.Errorf("قالب خروجی نامعتبر است: %w", err)
	}

	data := invoicehtml.PreviewData{Document: doc, Totals: totals, Template: template}
	if strings.TrimSpace(doc.CustomerSnapshot) != "" {
		if err := json.Unmarshal([]byte(doc.CustomerSnapshot), &data.Customer); err != nil {
			return fmt.Errorf("snapshot مشتری نامعتبر است: %w", err)
		}
	}
	if strings.TrimSpace(doc.IssuerSnapshot) != "" {
		if err := json.Unmarshal([]byte(doc.IssuerSnapshot), &data.Issuer); err != nil {
			return fmt.Errorf("snapshot صادرکننده نامعتبر است: %w", err)
		}
	}
	if template.LetterheadID > 0 {
		letterheads, err := templates.Letterheads(ctx)
		if err != nil {
			return fmt.Errorf("خواندن سربرگ: %w", err)
		}
		found := false
		for _, letterhead := range letterheads {
			if letterhead.ID == template.LetterheadID {
				data.Letterhead = letterhead
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("سربرگ قالب انتخاب‌شده پیدا نشد")
		}
	}
	brandName, logoPath := loadInvoiceSetting(ctx, contacts, "invoice_brand_name"), loadInvoiceSetting(ctx, contacts, "invoice_logo_path")
	if strings.TrimSpace(brandName) != "" {
		data.Letterhead.Title = brandName
	}
	if strings.TrimSpace(logoPath) != "" {
		data.Letterhead.LogoPath = logoPath
	}
	if template.ShowBankAccounts {
		accounts, err := templates.BankAccounts(ctx, doc.IssuerProfileID)
		if err != nil {
			return fmt.Errorf("خواندن حساب‌های بانکی: %w", err)
		}
		if len(template.BankAccountIDs) > 0 {
			selected := make(map[int64]bool, len(template.BankAccountIDs))
			for _, id := range template.BankAccountIDs {
				selected[id] = true
			}
			for _, account := range accounts {
				if selected[account.ID] {
					data.BankAccounts = append(data.BankAccounts, account)
				}
			}
		} else {
			data.BankAccounts = accounts
		}
	}
	switch strings.ToLower(format) {
	case "xlsx":
		if err := invoiceexcel.ExportInvoice(data, outputPath); err != nil {
			return fmt.Errorf("تولید فایل Excel: %w", err)
		}
		return nil
	case "pdf":
		fontDir, fontName, err := resolvePDFFont(pdfFontDirectory, pdfFontName...)
		if err != nil {
			return err
		}
		if err := invoicepdf.ExportInvoice(data, outputPath, invoicepdf.Options{FontDirectory: fontDir, FontName: fontName}); err != nil {
			return fmt.Errorf("تولید فایل PDF: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("قالب خروجی پشتیبانی نمی‌شود: %s", format)
	}
}

// findPDFFont locates an actual Persian TTF shipped with the application.
func findPDFFont(extraRoots ...string) (string, string, error) {
	roots := append([]string{}, extraRoots...)
	roots = append(roots, "assets/font")
	if executable, err := os.Executable(); err == nil {
		base := filepath.Dir(executable)
		roots = append(roots, filepath.Join(base, "assets", "font"), filepath.Join(base, "assets", "font", "iransans"))
	}
	if working, err := os.Getwd(); err == nil {
		roots = append(roots, filepath.Join(working, "assets", "font"))
	}
	seen := make(map[string]bool)
	for _, root := range roots {
		absolute, err := filepath.Abs(root)
		if err != nil || seen[absolute] {
			continue
		}
		seen[absolute] = true
		info, err := os.Stat(absolute)
		if err != nil || !info.IsDir() {
			continue
		}
		var foundName, foundDir string
		_ = filepath.WalkDir(absolute, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() {
				return nil
			}
			if strings.EqualFold(filepath.Ext(path), ".ttf") && strings.EqualFold(filepath.Base(path), "IRANSansX-Regular.ttf") {
				foundName, foundDir = filepath.Base(path), filepath.Dir(path)
				return filepath.SkipAll
			}
			return nil
		})
		if foundName != "" {
			return foundDir, foundName, nil
		}
	}
	return "", "", fmt.Errorf("فونت فارسی TTF پیدا نشد؛ فایل IRANSansX-Regular.ttf در assets/font موجود نیست")
}
func resolvePDFFont(directory string, names ...string) (string, string, error) {
	if len(names) > 0 && strings.TrimSpace(names[0]) != "" {
		return directory, names[0], nil
	}
	dir, name, err := findPDFFont(directory)
	if err != nil {
		return "", "", err
	}
	return dir, name, nil
}

func showInvoicePaymentWindow(parent *qt.QWidget, item domain.InvoiceListItem, invoices *application.Invoices, ctx context.Context, onSaved func()) {
	window := qt.NewQMainWindow2()
	window.QWidget.SetWindowTitle("ثبت دریافت — تخصیص به فاکتورها")
	window.QWidget.SetMinimumSize2(720, 620)
	container := qt.NewQWidget(window.QWidget)
	layout := qt.NewQVBoxLayout2()
	layout.SetContentsMargins(20, 18, 20, 18)
	container.SetLayout(layout.QLayout)
	currency := domain.CurrencyToman
	if item.Currency == "IRR" {
		currency = domain.CurrencyRial
	}
	title := qt.NewQLabel5("ثبت دریافت و تخصیص به فاکتورهای هم‌واحد", container)
	title.SetStyleSheet("font-size:15pt;font-weight:700;color:#102a56;")
	layout.AddWidget(title.QWidget)
	layout.AddWidget(qt.NewQLabel5("برای هر فاکتور مبلغ تخصیص را وارد کنید؛ صفر یعنی بدون تخصیص. جمع مبالغ باید با مبلغ دریافت برابر باشد.", container).QWidget)
	list, listErr := invoices.List(ctx, "")
	if listErr != nil {
		layout.AddWidget(qt.NewQLabel5("خواندن فهرست فاکتورها ناموفق بود: "+listErr.Error(), container).QWidget)
		window.SetCentralWidget(container)
		window.Show()
		return
	}
	scroll := qt.NewQScrollArea(container)
	scroll.SetWidgetResizable(true)
	rows := qt.NewQWidget(scroll.QWidget)
	rowsLayout := qt.NewQVBoxLayout2()
	rows.SetLayout(rowsLayout.QLayout)
	type allocationInput struct {
		invoice domain.InvoiceListItem
		amount  *qt.QLineEdit
	}
	inputs := make([]allocationInput, 0, len(list))
	for _, candidate := range list {
		if candidate.Kind != "invoice" || candidate.Status != "issued" || candidate.Balance <= 0 || candidate.Currency != item.Currency {
			continue
		}
		row := qt.NewQWidget(rows)
		rowLayout := qt.NewQHBoxLayout2()
		rowLayout.SetContentsMargins(6, 4, 6, 4)
		row.SetLayout(rowLayout.QLayout)
		label := qt.NewQLabel5(fmt.Sprintf("%s · %s · مانده %s %s", candidate.Number, candidate.Customer, formatCustomerMoney(candidate.Balance), string(currency)), row)
		label.SetWordWrap(true)
		rowLayout.AddWidget(label.QWidget)
		field := qt.NewQLineEdit(row)
		field.SetPlaceholderText("مبلغ تخصیص (۰ یعنی هیچ)")
		field.SetMaximumWidth(190)
		rowLayout.AddWidget(field.QWidget)
		rowsLayout.AddWidget(row)
		inputs = append(inputs, allocationInput{invoice: candidate, amount: field})
	}
	if len(inputs) == 0 {
		rowsLayout.AddWidget(qt.NewQLabel5("فاکتور منتشرشدهٔ دارای مانده در این واحد پول وجود ندارد.", rows).QWidget)
	}
	scroll.SetWidget(rows)
	layout.AddWidget(scroll.QWidget)
	reference := qt.NewQLineEdit(container)
	reference.SetPlaceholderText("شناسه پیگیری (اختیاری؛ برای جلوگیری از ثبت تکراری)")
	layout.AddWidget(reference.QWidget)
	method := qt.NewQLineEdit(container)
	method.SetPlaceholderText("روش پرداخت، مانند کارت‌به‌کارت یا نقدی")
	layout.AddWidget(method.QWidget)
	note := qt.NewQLineEdit(container)
	note.SetPlaceholderText("توضیحات (اختیاری)")
	layout.AddWidget(note.QWidget)
	status := qt.NewQLabel5("", container)
	status.SetWordWrap(true)
	layout.AddWidget(status.QWidget)
	save := qt.NewQPushButton5("ثبت دریافت", container)
	save.OnClicked(func() {
		allocations := make([]domain.PaymentAllocation, 0, len(inputs))
		var total int64
		for _, input := range inputs {
			raw := normalizeInvoiceDigits(strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(input.amount.Text(), ",", ""), "٬", "")))
			if raw == "" {
				continue
			}
			value, parseErr := strconv.ParseInt(raw, 10, 64)
			if parseErr != nil || value < 0 {
				status.SetText("مبلغ تخصیص باید عدد صحیح نامنفی باشد.")
				return
			}
			if value == 0 {
				continue
			}
			if value > input.invoice.Balance {
				status.SetText("تخصیص از ماندهٔ فاکتور " + input.invoice.Number + " بیشتر است.")
				return
			}
			if total > int64(^uint64(0)>>1)-value {
				status.SetText("جمع تخصیص‌ها بیش از محدودهٔ مجاز است.")
				return
			}
			total += value
			allocations = append(allocations, domain.PaymentAllocation{InvoiceID: input.invoice.ID, Amount: domain.Money(value)})
		}
		if total <= 0 {
			status.SetText("حداقل یک مبلغ تخصیص مثبت وارد کنید.")
			return
		}
		payment := domain.Payment{ExternalRef: strings.TrimSpace(reference.Text()), Amount: domain.Money(total), Currency: currency, PaidAt: time.Now(), Method: strings.TrimSpace(method.Text()), Note: strings.TrimSpace(note.Text())}
		if _, saveErr := invoices.RegisterPayment(ctx, payment, allocations); saveErr != nil {
			status.SetText("ثبت دریافت انجام نشد: " + saveErr.Error())
			return
		}
		status.SetText("دریافت و تخصیص‌ها با موفقیت ثبت شدند.")
		if onSaved != nil {
			onSaved()
		}
		window.Close()
	})
	layout.AddWidget(save.QWidget)
	window.SetCentralWidget(container)
	window.Show()
}
func showInvoicePaymentHistoryWindow(parent *qt.QWidget, invoices *application.Invoices, ctx context.Context) {
	window := qt.NewQMainWindow2()
	window.QWidget.SetWindowTitle("دفتر دریافت‌ها")
	window.QWidget.SetMinimumSize2(820, 600)
	root := qt.NewQWidget(window.QWidget)
	layout := qt.NewQVBoxLayout2()
	layout.SetContentsMargins(20, 18, 20, 18)
	root.SetLayout(layout.QLayout)
	title := qt.NewQLabel5("سوابق دریافت و تخصیص به فاکتورها", root)
	title.SetStyleSheet("font-size:15pt;font-weight:700;color:#102a56;")
	layout.AddWidget(title.QWidget)
	history, err := invoices.PaymentHistory(ctx)
	if err != nil {
		layout.AddWidget(qt.NewQLabel5("خواندن دفتر دریافت‌ها ناموفق بود: "+err.Error(), root).QWidget)
	} else if len(history) == 0 {
		layout.AddWidget(qt.NewQLabel5("هنوز دریافتی ثبت نشده است.", root).QWidget)
	} else {
		scroll := qt.NewQScrollArea(root)
		scroll.SetWidgetResizable(true)
		list := qt.NewQWidget(scroll.QWidget)
		listLayout := qt.NewQVBoxLayout2()
		listLayout.SetSpacing(10)
		list.SetLayout(listLayout.QLayout)
		for _, entry := range history {
			record := entry
			card := qt.NewQWidget(list)
			setInvoiceObjectName(card, "invoiceCard")
			cardLayout := qt.NewQVBoxLayout2()
			cardLayout.SetContentsMargins(14, 12, 14, 12)
			card.SetLayout(cardLayout.QLayout)
			jalaliDate := record.Payment.PaidAt.Format("2006-01-02")
			if j, convErr := domain.GregorianToJalali(record.Payment.PaidAt.Year(), int(record.Payment.PaidAt.Month()), record.Payment.PaidAt.Day()); convErr == nil {
				jalaliDate = fmt.Sprintf("%04d/%02d/%02d", j.Year, j.Month, j.Day)
			}
			currency := "تومان"
			if record.Payment.Currency == domain.CurrencyRial {
				currency = "ریال"
			}
			details := fmt.Sprintf("%s · %s · %s %s", jalaliDate, record.Payment.Method, formatCustomerMoney(int64(record.Payment.Amount)), currency)
			if record.Payment.ExternalRef != "" {
				details += " · پیگیری: " + record.Payment.ExternalRef
			}
			cardLayout.AddWidget(qt.NewQLabel5(details, card).QWidget)
			if record.ReversedAt.IsZero() {
				for _, allocation := range record.Allocations {
					cardLayout.AddWidget(qt.NewQLabel5(fmt.Sprintf("تخصیص به فاکتور %s: %s %s", allocation.InvoiceNumber, formatCustomerMoney(int64(allocation.Amount)), currency), card).QWidget)
				}
			} else {
				reversed := qt.NewQLabel5("ابطال‌شده از فضای وب · "+record.ReversalReason, card)
				reversed.SetStyleSheet("color:#a33a24;font-weight:700;")
				cardLayout.AddWidget(reversed.QWidget)
			}
			if record.Payment.Note != "" {
				cardLayout.AddWidget(qt.NewQLabel5(record.Payment.Note, card).QWidget)
			}
			listLayout.AddWidget(card)
		}
		listLayout.AddStretch()
		scroll.SetWidget(list)
		layout.AddWidget(scroll.QWidget)
	}
	close := qt.NewQPushButton5("بستن", root)
	close.OnClicked(func() { window.Close() })
	layout.AddWidget(close.QWidget)
	window.SetCentralWidget(root)
	window.Show()
}

func normalizeInvoiceDigits(value string) string {
	var out strings.Builder
	for _, r := range value {
		switch {
		case r >= '۰' && r <= '۹':
			out.WriteRune('0' + (r - '۰'))
		case r >= '٠' && r <= '٩':
			out.WriteRune('0' + (r - '٠'))
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}

// setInvoiceObjectName adapts strings to the value-type QAnyStringView used by miqt v0.15.0.
func setInvoiceObjectName(object interface{ SetObjectName(qt.QAnyStringView) }, name string) {
	view := qt.NewQAnyStringView3(name)
	object.SetObjectName(*view)
	view.Delete()
}
