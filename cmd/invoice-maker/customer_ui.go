package main

import (
	"context"
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
		preview := qt.NewQPushButton5("مشاهده فاکتور "+inv.Number, container)
		preview.OnClicked(func() {
			path, err := cachedInvoicePDF(ctx, record.ID, contacts, invoiceService, templates)
			if err != nil {
				qt.QMessageBox_Critical(window.QWidget, "نمایش فاکتور ناموفق بود", err.Error())
				return
			}
			showInvoicePDFPreview(window.QWidget, path, record.Number)
		})
		layout.AddWidget(preview.QWidget)
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
