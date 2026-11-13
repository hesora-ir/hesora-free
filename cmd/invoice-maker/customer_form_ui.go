package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/hesora-ir/hesora-free/internal/application"
	"github.com/hesora-ir/hesora-free/internal/domain"
	qt "github.com/mappu/miqt/qt6"
)

// openCustomerForm is shared by customer management and invoice issuance.
// The callback receives the saved profile so the caller can refresh its list.
func openCustomerForm(parent *qt.QWidget, contacts *application.Contacts, ctx context.Context, existing *domain.Customer, onSaved func(domain.Customer)) {
	window := qt.NewQMainWindow2()
	if existing == nil {
		window.QWidget.SetWindowTitle("مشتری جدید")
	} else {
		window.QWidget.SetWindowTitle("ویرایش مشتری")
	}
	window.QWidget.SetMinimumSize2(680, 600)
	container := qt.NewQWidget(window.QWidget)
	layout := qt.NewQVBoxLayout2()
	layout.SetContentsMargins(24, 22, 24, 22)
	layout.SetSpacing(12)
	container.SetLayout(layout.QLayout)
	title := "ثبت مشتری جدید"
	if existing != nil {
		title = "ویرایش اطلاعات مشتری"
	}
	layout.AddWidget(sectionHeading(container, title))
	layout.AddWidget(qt.NewQLabel5("اطلاعات پایه و راه‌های تماس مشتری را وارد کنید.", container).QWidget)
	typeRow := qt.NewQHBoxLayout2()
	individual := qt.NewQPushButton5("حقیقی", container)
	company := qt.NewQPushButton5("حقوقی", container)
	typeRow.AddWidget(individual.QWidget)
	typeRow.AddWidget(company.QWidget)
	layout.AddWidget(qt.NewQLabel5("نوع مشتری", container).QWidget)
	layout.AddLayout(typeRow.QLayout)
	identity := qt.NewQHBoxLayout2()
	first, last := qt.NewQLineEdit(container), qt.NewQLineEdit(container)
	first.SetPlaceholderText("نام")
	last.SetPlaceholderText("نام خانوادگی")
	identity.AddWidget(first.QWidget)
	identity.AddWidget(last.QWidget)
	layout.AddLayout(identity.QLayout)
	legal := qt.NewQLineEdit(container)
	legal.SetPlaceholderText("نام شرکت / شخص حقوقی")
	layout.AddWidget(legal.QWidget)
	tax := qt.NewQLineEdit(container)
	tax.SetPlaceholderText("شناسه ملی یا شماره اقتصادی (اختیاری)")
	layout.AddWidget(tax.QWidget)
	email := qt.NewQLineEdit(container)
	email.SetPlaceholderText("ایمیل (اختیاری)")
	layout.AddWidget(email.QWidget)
	phoneLayout := qt.NewQVBoxLayout2()
	layout.AddWidget(sectionHeading(container, "شماره‌های تماس"))
	layout.AddLayout(phoneLayout.QLayout)
	phones := make([]*qt.QLineEdit, 0)
	addPhone := func() {
		f := qt.NewQLineEdit(container)
		f.SetPlaceholderText(fmt.Sprintf("شماره تماس %d", len(phones)+1))
		phoneLayout.AddWidget(f.QWidget)
		phones = append(phones, f)
	}
	addPhoneButton := qt.NewQPushButton5("+ افزودن شماره تماس", container)
	addPhoneButton.OnClicked(addPhone)
	layout.AddWidget(addPhoneButton.QWidget)
	addPhone()
	addressLayout := qt.NewQVBoxLayout2()
	layout.AddWidget(sectionHeading(container, "نشانی‌ها"))
	layout.AddLayout(addressLayout.QLayout)
	addresses := make([]*qt.QLineEdit, 0)
	addAddress := func() {
		f := qt.NewQLineEdit(container)
		f.SetPlaceholderText(fmt.Sprintf("نشانی %d", len(addresses)+1))
		addressLayout.AddWidget(f.QWidget)
		addresses = append(addresses, f)
	}
	addAddressButton := qt.NewQPushButton5("+ افزودن نشانی", container)
	addAddressButton.OnClicked(addAddress)
	layout.AddWidget(addAddressButton.QWidget)
	addAddress()
	status := qt.NewQLabel5("", container)
	status.SetWordWrap(true)
	layout.AddWidget(status.QWidget)
	actions := qt.NewQHBoxLayout2()
	save := qt.NewQPushButton5("ذخیره مشتری", container)
	cancel := qt.NewQPushButton5("بستن", container)
	actions.AddWidget(save.QWidget)
	actions.AddWidget(cancel.QWidget)
	layout.AddLayout(actions.QLayout)
	kind := domain.CustomerTypeIndividual
	setKind := func(k string) {
		kind = k
		first.SetEnabled(k == domain.CustomerTypeIndividual)
		last.SetEnabled(k == domain.CustomerTypeIndividual)
		legal.SetEnabled(k == domain.CustomerTypeCompany)
		tax.SetEnabled(k == domain.CustomerTypeCompany)
	}
	individual.OnClicked(func() { setKind(domain.CustomerTypeIndividual) })
	company.OnClicked(func() { setKind(domain.CustomerTypeCompany) })
	if existing != nil {
		kind = existing.Type
		first.SetText(existing.FirstName)
		last.SetText(existing.LastName)
		legal.SetText(existing.LegalName)
		if kind == domain.CustomerTypeIndividual && first.Text() == "" && last.Text() == "" {
			first.SetText(existing.Name)
		}
		if kind == domain.CustomerTypeCompany && legal.Text() == "" {
			legal.SetText(existing.Name)
		}
		tax.SetText(existing.TaxID)
		email.SetText(existing.Email)
		if kind == domain.CustomerTypeCompany {
			setKind(kind)
		}
		for len(phones) < len(existing.Phones) {
			addPhone()
		}
		for i, v := range existing.Phones {
			phones[i].SetText(v)
		}
		for len(addresses) < len(existing.Addresses) {
			addAddress()
		}
		for i, v := range existing.Addresses {
			addresses[i].SetText(v)
		}
	}
	setKind(kind)
	cancel.OnClicked(func() { window.Close() })
	save.OnClicked(func() {
		c := domain.Customer{Type: kind, FirstName: strings.TrimSpace(first.Text()), LastName: strings.TrimSpace(last.Text()), LegalName: strings.TrimSpace(legal.Text()), TaxID: strings.TrimSpace(tax.Text()), Email: strings.TrimSpace(email.Text())}
		if existing != nil {
			c.ID = existing.ID
		}
		if kind == domain.CustomerTypeIndividual {
			c.Name = strings.TrimSpace(c.FirstName + " " + c.LastName)
		} else {
			c.Name = c.LegalName
		}
		for _, f := range phones {
			if v := strings.TrimSpace(f.Text()); v != "" {
				c.Phones = append(c.Phones, v)
			}
		}
		for _, f := range addresses {
			if v := strings.TrimSpace(f.Text()); v != "" {
				c.Addresses = append(c.Addresses, v)
			}
		}
		var err error
		if c.ID == 0 {
			c.ID, err = contacts.CreateCustomer(ctx, c)
		} else {
			err = contacts.UpdateCustomer(ctx, c)
		}
		if err != nil {
			status.SetText("ذخیره مشتری انجام نشد: " + err.Error())
			return
		}
		if onSaved != nil {
			onSaved(c)
		}
		window.Close()
	})
	scroll := qt.NewQScrollArea(window.QWidget)
	scroll.SetWidgetResizable(true)
	scroll.SetWidget(container)
	window.SetCentralWidget(scroll.QWidget)
	window.Show()
	_ = parent
}
