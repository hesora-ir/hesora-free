package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hesora-ir/hesora-free/internal/application"
	"github.com/hesora-ir/hesora-free/internal/domain"
	qt "github.com/mappu/miqt/qt6"
)

func buildCustomerPage(parent *qt.QWidget, contacts *application.Contacts, invoiceService *application.Invoices, templates *application.Templates, ctx context.Context) page {
	p := newPage(parent, "مشتریان", "مدیریت اطلاعات تماس، فاکتورها و مانده‌حساب مشتریان")
	header := qt.NewQHBoxLayout2()
	search := qt.NewQLineEdit(p.widget)
	search.SetPlaceholderText("جست‌وجو بر اساس نام، تماس، ایمیل یا شناسه اقتصادی")
	searchButton := qt.NewQPushButton5("جست‌وجو", p.widget)
	addButton := qt.NewQPushButton5("+ مشتری جدید", p.widget)
	setInvoiceObjectName(addButton, "primaryAction")
	header.AddWidget(search.QWidget)
	header.AddWidget(searchButton.QWidget)
	header.AddWidget(addButton.QWidget)
	p.layout.AddLayout(header.QLayout)
	status := qt.NewQLabel5("", p.widget)
	status.SetWordWrap(true)
	setInvoiceObjectName(status, "muted")
	p.layout.AddWidget(status.QWidget)
	listPanel, listLayout := newSurface(p.widget, "فهرست مشتریان", "برای ویرایش یا مشاهدهٔ سوابق، مشتری را انتخاب کنید.")
	p.layout.AddWidget(listPanel)
	cardLayout := qt.NewQVBoxLayout2()
	listLayout.AddLayout(cardLayout.QLayout)
	oldCards := make([]*qt.QWidget, 0)
	var refresh func()
	openNew := func() {
		openCustomerForm(p.widget, contacts, ctx, nil, func(domain.Customer) { refresh(); status.SetText("مشتری با موفقیت ثبت شد.") })
	}
	refresh = func() {
		for _, card := range oldCards {
			card.SetVisible(false)
		}
		oldCards = oldCards[:0]
		customers, err := contacts.SearchCustomers(ctx, search.Text())
		if err != nil {
			status.SetText("خطا در جست‌وجوی مشتریان: " + err.Error())
			return
		}
		for _, item := range customers {
			c := item
			card := qt.NewQWidget(listPanel)
			setInvoiceObjectName(card, "surface")
			lay := qt.NewQVBoxLayout2()
			lay.SetContentsMargins(16, 14, 16, 14)
			lay.SetSpacing(7)
			card.SetLayout(lay.QLayout)
			typeName := "حقیقی"
			if c.Type == domain.CustomerTypeCompany {
				typeName = "حقوقی"
			}
			title := qt.NewQLabel5(fmt.Sprintf("%s  ·  %s  ·  شناسه %d", c.Name, typeName, c.ID), card)
			setInvoiceObjectName(title, "sectionTitle")
			lay.AddWidget(title.QWidget)
			phones := strings.Join(c.Phones, "، ")
			if phones == "" {
				phones = "ثبت نشده"
			}
			lay.AddWidget(qt.NewQLabel5("تماس: "+phones, card).QWidget)
			addresses := strings.Join(c.Addresses, " | ")
			if addresses == "" {
				addresses = "ثبت نشده"
			}
			address := qt.NewQLabel5("نشانی: "+addresses, card)
			address.SetWordWrap(true)
			lay.AddWidget(address.QWidget)
			stats := fmt.Sprintf("فاکتورهای صادرشده: %d  ·  باز: %d  ·  تسویه‌شده: %d  ·  پیش‌فاکتور: %d", c.IssuedInvoiceCount, c.OpenInvoiceCount, c.SettledInvoiceCount, c.ProformaCount)
			lay.AddWidget(qt.NewQLabel5(stats, card).QWidget)
			debt := qt.NewQLabel5("مانده: "+formatCustomerMoney(c.DebtIRT)+" تومان  |  "+formatCustomerMoney(c.DebtIRR)+" ریال", card)
			setInvoiceObjectName(debt, "invoiceAmount")
			lay.AddWidget(debt.QWidget)
			actions := qt.NewQHBoxLayout2()
			edit := qt.NewQPushButton5("ویرایش", card)
			history := qt.NewQPushButton5("نمایش فاکتورها", card)
			edit.OnClicked(func() {
				selected := c
				openCustomerForm(p.widget, contacts, ctx, &selected, func(domain.Customer) { refresh(); status.SetText("تغییرات مشتری ذخیره شد.") })
			})
			history.OnClicked(func() { showCustomerInvoices(p.widget, contacts, invoiceService, templates, ctx, c, status) })
			actions.AddWidget(edit.QWidget)
			actions.AddWidget(history.QWidget)
			lay.AddLayout(actions.QLayout)
			cardLayout.AddWidget(card)
			oldCards = append(oldCards, card)
		}
		if len(customers) == 0 {
			status.SetText("مشتری‌ای پیدا نشد.")
		}
	}
	addButton.OnClicked(openNew)
	searchButton.OnClicked(refresh)
	search.OnReturnPressed(refresh)
	refresh()
	p.layout.AddStretch()
	scroll := qt.NewQScrollArea(parent)
	scroll.SetWidgetResizable(true)
	scroll.SetWidget(p.widget)
	return page{widget: scroll.QWidget, layout: p.layout}
}

func showCustomerInvoices(parent *qt.QWidget, contacts *application.Contacts, invoiceService *application.Invoices, templates *application.Templates, ctx context.Context, c domain.Customer, status *qt.QLabel) {
	invoices, err := contacts.CustomerInvoices(ctx, c.ID)
	if err != nil {
		status.SetText("خواندن فاکتورهای مشتری ناموفق بود: " + err.Error())
		return
	}
	window := qt.NewQMainWindow2()
	window.QWidget.SetLayoutDirection(qt.RightToLeft)
	window.QWidget.SetWindowTitle("فاکتورهای مشتری — " + c.Name)
	window.QWidget.SetMinimumSize2(760, 520)
	window.QWidget.SetStyleSheet(`QMainWindow, QWidget { background:#f4f7fb; color:#172b4d; font-family:"Segoe UI","Tahoma"; }
QLabel#surface { background:white; border:1px solid #dfe7f1; border-radius:12px; padding:14px 16px; color:#263b59; }
QPushButton { background:#7058ed; color:white; border:0; border-radius:8px; padding:9px 13px; font-weight:600; }
QPushButton#secondaryAction { background:#edf3fb; color:#23456f; border:1px solid #d8e3f1; }
QPushButton#primaryAction { background:#7058ed; color:white; }`)
	container := qt.NewQWidget(window.QWidget)
	layout := qt.NewQVBoxLayout2()
	layout.SetContentsMargins(22, 20, 22, 20)
	layout.SetSpacing(10)
	container.SetLayout(layout.QLayout)
	layout.AddWidget(sectionHeading(container, "فاکتورهای "+c.Name))
	if len(invoices) == 0 {
		layout.AddWidget(qt.NewQLabel5("برای این مشتری هنوز سندی ثبت نشده است.", container).QWidget)
	}
	for _, inv := range invoices {
		record := inv
		currency := "تومان"
		if inv.Currency == domain.CurrencyRial {
			currency = "ریال"
		}
		documentType, state := "فاکتور", "پیش‌نویس"
		if inv.Kind == "proforma" {
			documentType = "پیش‌فاکتور"
		}
		if inv.Status == "issued" {
			state = "پرداخت‌نشده"
			if inv.Balance == 0 {
				state = "تسویه‌شده"
			} else if inv.Paid > 0 {
				state = "پرداخت بخشی"
			}
		}
		line := fmt.Sprintf("%s %s  ·  %s  ·  %s\nمبلغ %s %s  ·  پرداخت‌شده %s  ·  مانده %s", documentType, inv.Number, inv.IssueDate, state, formatCustomerMoney(int64(inv.Total)), currency, formatCustomerMoney(int64(inv.Paid)), formatCustomerMoney(int64(inv.Balance)))
		label := qt.NewQLabel5(line, container)
		label.SetWordWrap(true)
		setInvoiceObjectName(label, "surface")
		layout.AddWidget(label.QWidget)
		actions := qt.NewQHBoxLayout2()
		preview := qt.NewQPushButton5("پیش‌نمایش", container)
		preview.OnClicked(func() {
			path, err := cachedInvoicePDF(ctx, record.ID, contacts, invoiceService, templates)
			if err != nil {
				qt.QMessageBox_Critical(window.QWidget, "نمایش فاکتور ناموفق بود", err.Error())
				return
			}
			showInvoicePDFPreview(window.QWidget, path, record.Number)
		})
		actions.AddWidget(preview.QWidget)
		if record.Status == "draft" || (record.Status == "issued" && record.Balance > 0) {
			edit := qt.NewQPushButton5("ویرایش", container)
			setInvoiceObjectName(edit, "secondaryAction")
			edit.OnClicked(func() {
				doc, err := invoiceService.Get(ctx, record.ID)
				if err != nil {
					qt.QMessageBox_Critical(window.QWidget, "بازکردن سند ناموفق بود", err.Error())
					return
				}
				editorWindow := qt.NewQMainWindow2()
				editorWindow.QWidget.SetLayoutDirection(qt.RightToLeft)
				editorWindow.QWidget.SetWindowTitle("ویرایش سند " + record.Number)
				editorWindow.QWidget.SetMinimumSize2(960, 700)
				editor := buildInvoicePage(editorWindow.QWidget, contacts, invoiceService, ctx, &doc, invoiceEditorOptions{templates: templates, contacts: contacts})
				editorWindow.SetCentralWidget(editor.widget)
				editorWindow.Show()
			})
			actions.AddWidget(edit.QWidget)
		}
		pdf := qt.NewQPushButton5("خروجی PDF", container)
		pdf.OnClicked(func() {
			path := qt.QFileDialog_GetSaveFileName4(window.QWidget, "ذخیره فاکتور به PDF", record.Number+".pdf", "PDF (*.pdf)")
			if path == "" {
				return
			}
			if !strings.HasSuffix(strings.ToLower(path), ".pdf") {
				path += ".pdf"
			}
			dir, name, err := findPDFFont()
			if err == nil {
				err = exportInvoiceFile(ctx, record.ID, path, "pdf", contacts, invoiceService, templates, dir, name)
			}
			if err != nil {
				qt.QMessageBox_Critical(window.QWidget, "خروجی PDF ناموفق بود", err.Error())
				return
			}
			qt.QMessageBox_Information(window.QWidget, "خروجی آماده شد", "فایل PDF ذخیره شد:\n"+path)
		})
		xlsx := qt.NewQPushButton5("خروجی Excel", container)
		xlsx.OnClicked(func() {
			path := qt.QFileDialog_GetSaveFileName4(window.QWidget, "ذخیره فاکتور به Excel", record.Number+".xlsx", "Excel Workbook (*.xlsx)")
			if path == "" {
				return
			}
			if !strings.HasSuffix(strings.ToLower(path), ".xlsx") {
				path += ".xlsx"
			}
			if err := exportInvoiceFile(ctx, record.ID, path, "xlsx", contacts, invoiceService, templates, ""); err != nil {
				qt.QMessageBox_Critical(window.QWidget, "خروجی Excel ناموفق بود", err.Error())
				return
			}
			qt.QMessageBox_Information(window.QWidget, "خروجی آماده شد", "فایل Excel ذخیره شد:\n"+path)
		})
		actions.AddWidget(pdf.QWidget)
		actions.AddWidget(xlsx.QWidget)
		share := qt.NewQPushButton5("اشتراک‌گذاری", container)
		setInvoiceObjectName(share, "primaryAction")
		share.OnClicked(func() {
			doc, err := invoiceService.Get(ctx, record.ID)
			if err != nil {
				qt.QMessageBox_Critical(window.QWidget, "خواندن فاکتور ناموفق بود", err.Error())
				return
			}
			var snapshot invoicePartySnapshot
			_ = json.Unmarshal([]byte(doc.CustomerSnapshot), &snapshot)
			showInvoiceShareOptions(window.QWidget, doc, snapshot.Phone, ctx, contacts, invoiceService, templates)
		})
		actions.AddWidget(share.QWidget)
		layout.AddLayout(actions.QLayout)
	}
	layout.AddStretch()
	scroll := qt.NewQScrollArea(window.QWidget)
	scroll.SetWidgetResizable(true)
	scroll.SetWidget(container)
	window.SetCentralWidget(scroll.QWidget)
	window.Show()
	_ = parent
}

func formatCustomerMoney(value int64) string {
	raw := fmt.Sprintf("%d", value)
	var out strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			out.WriteRune(r + ('۰' - '0'))
		} else {
			out.WriteRune(r)
		}
	}
	return out.String()
}
