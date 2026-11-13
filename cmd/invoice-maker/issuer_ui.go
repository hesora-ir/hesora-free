package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/hesora-ir/hesora-free/internal/application"
	"github.com/hesora-ir/hesora-free/internal/banking"
	"github.com/hesora-ir/hesora-free/internal/domain"
	qt "github.com/mappu/miqt/qt6"
)

type issuerBankRow struct {
	bankName *qt.QLabel
	card     *qt.QLineEdit
	account  *qt.QLineEdit
	iban     *qt.QLineEdit
}

func buildIssuerPage(parent *qt.QWidget, contacts *application.Contacts, ctx context.Context) page {
	p := newPage(parent, "فروشگاه", "مدیریت اطلاعات فروشگاه و برند فاکتور")
	add := qt.NewQPushButton5("+ افزودن فروشگاه", p.widget)
	setInvoiceObjectName(add, "primaryAction")
	p.layout.AddWidget(add.QWidget)
	list, listLayout := newSurface(p.widget, "فروشگاه شما", "نسخهٔ رایگان: یک فروشگاه، ۵۰ مشتری و ۲۰۰ فاکتور.")
	p.layout.AddWidget(list)
	profiles, err := contacts.IssuerProfiles(ctx)
	if err == nil {
		if len(profiles) > 0 {
			add.SetText("مدیریت فروشگاه")
		}
		for _, profile := range profiles {
			label := profile.Name
			if profile.Type == domain.IssuerTypeCompany {
				label = profile.LegalName
			}
			listLayout.AddWidget(qt.NewQLabel5(fmt.Sprintf("%s  ·  شناسه %d", label, profile.ID), list).QWidget)
		}
	} else {
		listLayout.AddWidget(qt.NewQLabel5("خواندن پروفایل‌ها ناموفق بود: "+err.Error(), list).QWidget)
	}
	add.OnClicked(func() {
		window := qt.NewQMainWindow2()
		window.QWidget.SetWindowTitle("فروشگاه Hesora")
		window.QWidget.SetMinimumSize2(820, 700)
		form := buildIssuerFormPage(window.QWidget, contacts, ctx)
		window.SetCentralWidget(form.widget)
		window.Show()
	})
	p.layout.AddStretch()
	return p
}

func buildIssuerFormPage(parent *qt.QWidget, contacts *application.Contacts, ctx context.Context) page {
	p := newPage(parent, "فروشگاه", "اطلاعات فروشگاه و صادرکننده را ثبت کنید")
	p.layout.AddWidget(sectionHeading(p.widget, "برند فاکتور"))
	brandName := qt.NewQLineEdit(p.widget)
	brandName.SetPlaceholderText("نام برند در سربرگ فاکتور")
	brandName.SetText(loadInvoiceSetting(ctx, contacts, "invoice_brand_name"))
	p.layout.AddWidget(brandName.QWidget)
	logoPath := qt.NewQLineEdit(p.widget)
	logoPath.SetPlaceholderText("مسیر فایل لوگو")
	logoPath.SetText(loadInvoiceSetting(ctx, contacts, "invoice_logo_path"))
	logoRow := qt.NewQHBoxLayout2()
	chooseLogo := qt.NewQPushButton5("انتخاب لوگو…", p.widget)
	logoRow.AddWidget(logoPath.QWidget)
	logoRow.AddWidget(chooseLogo.QWidget)
	p.layout.AddLayout(logoRow.QLayout)
	chooseLogo.OnClicked(func() {
		path := qt.QFileDialog_GetOpenFileName4(p.widget, "انتخاب لوگو", "", "تصویر (*.png *.jpg *.jpeg)")
		if path != "" {
			logoPath.SetText(path)
		}
	})
	typeStatus := qt.NewQLabel5("نوع صادرکننده را انتخاب کنید.", p.widget)
	p.layout.AddWidget(typeStatus.QWidget)
	typeButtons := qt.NewQHBoxLayout2()
	individualButton := qt.NewQPushButton5("حقیقی", p.widget)
	companyButton := qt.NewQPushButton5("حقوقی", p.widget)
	typeButtons.AddWidget(individualButton.QWidget)
	typeButtons.AddWidget(companyButton.QWidget)
	p.layout.AddLayout(typeButtons.QLayout)

	individualPanel := qt.NewQWidget(p.widget)
	individualLayout := qt.NewQVBoxLayout2()
	individualLayout.SetContentsMargins(0, 0, 0, 0)
	individualPanel.SetLayout(individualLayout.QLayout)
	firstName := qt.NewQLineEdit(individualPanel)
	firstName.SetPlaceholderText("نام")
	lastName := qt.NewQLineEdit(individualPanel)
	lastName.SetPlaceholderText("نام خانوادگی")
	individualLayout.AddWidget(firstName.QWidget)
	individualLayout.AddWidget(lastName.QWidget)
	p.layout.AddWidget(individualPanel)

	companyPanel := qt.NewQWidget(p.widget)
	companyLayout := qt.NewQVBoxLayout2()
	companyLayout.SetContentsMargins(0, 0, 0, 0)
	companyPanel.SetLayout(companyLayout.QLayout)
	companyName := qt.NewQLineEdit(companyPanel)
	companyName.SetPlaceholderText("نام شرکت")
	taxID := qt.NewQLineEdit(companyPanel)
	taxID.SetPlaceholderText("شماره اقتصادی")
	companyLayout.AddWidget(companyName.QWidget)
	companyLayout.AddWidget(taxID.QWidget)
	p.layout.AddWidget(companyPanel)
	companyPanel.SetVisible(false)

	address := qt.NewQLineEdit(p.widget)
	address.SetPlaceholderText("آدرس مغازه / دفتر / شرکت")
	p.layout.AddWidget(address.QWidget)
	email := qt.NewQLineEdit(p.widget)
	email.SetPlaceholderText("ایمیل (اختیاری)")
	p.layout.AddWidget(email.QWidget)

	// Invoice footnotes are reusable selectable items.
	p.layout.AddWidget(sectionHeading(p.widget, "پی‌نوشت فاکتور"))
	footnoteDescription := qt.NewQLabel5("شرایط اعتبار، مدت تحویل، گارانتی، هزینه حمل و سایر توضیحات پیش‌فرض اسناد را وارد کنید.", p.widget)
	footnoteDescription.SetWordWrap(true)
	p.layout.AddWidget(footnoteDescription.QWidget)
	footnoteLayout := qt.NewQVBoxLayout2()
	p.layout.AddLayout(footnoteLayout.QLayout)
	footnoteRows := make([]*qt.QLineEdit, 0)
	addFootnote := func(value string) {
		field := qt.NewQLineEdit(p.widget)
		field.SetPlaceholderText("متن پی‌نوشت فاکتور")
		field.SetText(value)
		footnoteLayout.AddWidget(field.QWidget)
		footnoteRows = append(footnoteRows, field)
	}
	for _, item := range loadInvoiceFootnotes(ctx, contacts) {
		addFootnote(item)
	}
	if len(footnoteRows) == 0 {
		addFootnote("")
	}
	footnoteAdd := qt.NewQPushButton5("+ افزودن پی‌نوشت", p.widget)
	footnoteAdd.OnClicked(func() { addFootnote("") })
	p.layout.AddWidget(footnoteAdd.QWidget)
	footnoteStatus := qt.NewQLabel5("پی‌نوشت به‌صورت پیش‌فرض برای استفاده در فاکتورها نگهداری می‌شود.", p.widget)
	footnoteStatus.SetWordWrap(true)
	p.layout.AddWidget(footnoteStatus.QWidget)
	footnoteSaveButton := qt.NewQPushButton5("ذخیره پی‌نوشت", p.widget)
	footnoteSaveButton.OnClicked(func() {
		items := make([]string, 0, len(footnoteRows))
		for _, field := range footnoteRows {
			if value := strings.TrimSpace(field.Text()); value != "" {
				items = append(items, value)
			}
		}
		encoded, _ := json.Marshal(items)
		if err := contacts.SetSetting(ctx, domain.Setting{Key: "invoice_footnote", Value: string(encoded)}); err != nil {
			footnoteStatus.SetText("ذخیره پی‌نوشت انجام نشد: " + err.Error())
			return
		}
		footnoteStatus.SetText("پی‌نوشت پیش‌فرض فاکتورها ذخیره شد.")
	})
	p.layout.AddWidget(footnoteSaveButton.QWidget)

	p.layout.AddWidget(sectionHeading(p.widget, "شماره‌های تماس"))
	phoneLayout := qt.NewQVBoxLayout2()
	p.layout.AddLayout(phoneLayout.QLayout)
	phoneFields := make([]*qt.QLineEdit, 0)
	addPhoneRow := func() {
		row := qt.NewQHBoxLayout2()
		field := qt.NewQLineEdit(p.widget)
		field.SetPlaceholderText(fmt.Sprintf("شماره تماس %d", len(phoneFields)+1))
		row.AddWidget(field.QWidget)
		phoneLayout.AddLayout(row.QLayout)
		phoneFields = append(phoneFields, field)
	}
	addPhoneButton := qt.NewQPushButton5("+ افزودن شماره تماس", p.widget)
	addPhoneButton.OnClicked(addPhoneRow)
	p.layout.AddWidget(addPhoneButton.QWidget)
	addPhoneRow()

	p.layout.AddWidget(sectionHeading(p.widget, "حساب‌های بانکی"))
	accountHeader := qt.NewQHBoxLayout2()
	for _, heading := range []string{"شماره کارت", "شماره حساب", "شماره شبا"} {
		label := qt.NewQLabel5(heading, p.widget)
		accountHeader.AddWidget(label.QWidget)
	}
	p.layout.AddLayout(accountHeader.QLayout)
	accountLayout := qt.NewQVBoxLayout2()
	p.layout.AddLayout(accountLayout.QLayout)
	accountRows := make([]issuerBankRow, 0)
	addAccountRow := func() {
		row := qt.NewQHBoxLayout2()
		card := qt.NewQLineEdit(p.widget)
		card.SetLayoutDirection(qt.LeftToRight)
		card.SetPlaceholderText("شماره کارت")
		account := qt.NewQLineEdit(p.widget)
		account.SetLayoutDirection(qt.LeftToRight)
		account.SetPlaceholderText("شماره حساب")
		iban := qt.NewQLineEdit(p.widget)
		iban.SetLayoutDirection(qt.LeftToRight)
		iban.SetPlaceholderText("IR + ۲۴ رقم (یا فقط ۲۴ رقم)")
		iban.OnEditingFinished(func() {
			if result := banking.ValidateIBAN(iban.Text()); result.Valid {
				iban.SetText(result.Normalized)
			}
		})
		bankName := qt.NewQLabel5("بانک شناسایی نشده", p.widget)
		row.AddWidget(bankName.QWidget)
		row.AddWidget(card.QWidget)
		row.AddWidget(account.QWidget)
		row.AddWidget(iban.QWidget)
		accountLayout.AddLayout(row.QLayout)
		refreshBank := func(string) {
			detected := banking.DetectBankByCard(card.Text())
			if detected == nil {
				result := banking.ValidateIBAN(iban.Text())
				if result.Valid {
					detected = result.Bank
				}
			}
			if detected != nil {
				bankName.SetText(detected.Name)
			} else {
				bankName.SetText("بانک شناسایی نشده")
			}
		}
		card.OnTextChanged(refreshBank)
		iban.OnTextChanged(refreshBank)
		accountRows = append(accountRows, issuerBankRow{bankName: bankName, card: card, account: account, iban: iban})
	}
	addAccountButton := qt.NewQPushButton5("+ افزودن حساب", p.widget)
	addAccountButton.OnClicked(addAccountRow)
	p.layout.AddWidget(addAccountButton.QWidget)
	addAccountRow()

	status := qt.NewQLabel5("اطلاعات را تکمیل و ذخیره کنید.", p.widget)
	status.SetWordWrap(true)
	p.layout.AddWidget(status.QWidget)
	saveButton := qt.NewQPushButton5("ذخیره پروفایل", p.widget)
	newButton := qt.NewQPushButton5("پروفایل جدید", p.widget)
	actions := qt.NewQHBoxLayout2()
	actions.AddWidget(saveButton.QWidget)
	actions.AddWidget(newButton.QWidget)
	p.layout.AddLayout(actions.QLayout)

	p.layout.AddWidget(sectionHeading(p.widget, "پروفایل‌های ثبت‌شده — برای ویرایش روی نام کلیک کنید"))
	profileList := qt.NewQVBoxLayout2()
	p.layout.AddLayout(profileList.QLayout)
	profileButtons := make([]*qt.QPushButton, 0)

	profileType := domain.IssuerTypeIndividual
	var editingID int64
	setType := func(kind string) {
		profileType = kind
		individualPanel.SetVisible(kind == domain.IssuerTypeIndividual)
		companyPanel.SetVisible(kind == domain.IssuerTypeCompany)
		if kind == domain.IssuerTypeIndividual {
			typeStatus.SetText("پروفایل شخص حقیقی")
		} else {
			typeStatus.SetText("پروفایل شرکت / شخص حقوقی")
		}
	}
	individualButton.OnClicked(func() { setType(domain.IssuerTypeIndividual) })
	companyButton.OnClicked(func() { setType(domain.IssuerTypeCompany) })

	clearForm := func() {
		editingID = 0
		firstName.SetText("")
		lastName.SetText("")
		companyName.SetText("")
		taxID.SetText("")
		address.SetText("")
		email.SetText("")
		for _, field := range phoneFields {
			field.SetText("")
		}
		for _, row := range accountRows {
			row.card.SetText("")
			row.account.SetText("")
			row.iban.SetText("")
		}
		status.SetText("پروفایل جدید؛ نوع صادرکننده را انتخاب و اطلاعات را وارد کنید.")
		saveButton.SetText("ذخیره پروفایل")
	}
	loadProfile := func(profile domain.IssuerProfile) {
		editingID = profile.ID
		setType(profile.Type)
		firstName.SetText(profile.FirstName)
		lastName.SetText(profile.LastName)
		if profile.Type == domain.IssuerTypeIndividual && profile.FirstName == "" && profile.LastName == "" {
			firstName.SetText(profile.Name)
		}
		companyName.SetText(profile.LegalName)
		taxID.SetText(profile.TaxID)
		address.SetText(profile.Address)
		email.SetText(profile.Email)
		for len(phoneFields) < len(profile.Phones) {
			addPhoneRow()
		}
		for i, field := range phoneFields {
			field.SetText("")
			if i < len(profile.Phones) {
				field.SetText(profile.Phones[i])
			}
		}
		for len(accountRows) < len(profile.BankDetails) {
			addAccountRow()
		}
		for i, row := range accountRows {
			row.card.SetText("")
			row.account.SetText("")
			row.iban.SetText("")
			if i < len(profile.BankDetails) {
				row.card.SetText(profile.BankDetails[i].CardNumber)
				row.account.SetText(profile.BankDetails[i].AccountNumber)
				row.iban.SetText(profile.BankDetails[i].IBAN)
			}
		}
		saveButton.SetText("ذخیره تغییرات")
		status.SetText(fmt.Sprintf("پروفایل شماره %d برای ویرایش بارگذاری شد.", profile.ID))
	}

	refreshProfiles := func() {
		for _, oldButton := range profileButtons {
			oldButton.SetVisible(false)
		}
		profileButtons = profileButtons[:0]
		profiles, err := contacts.IssuerProfiles(ctx)
		if err != nil {
			status.SetText("خطا در خواندن پروفایل‌ها: " + err.Error())
			return
		}
		if len(profiles) == 0 {
			newButton.SetEnabled(true)
			status.SetText("هنوز پروفایلی ثبت نشده است.")
			return
		}
		newButton.SetEnabled(false)
		for _, item := range profiles {
			profile := item
			label := profile.Name
			if profile.Type == domain.IssuerTypeCompany {
				label = "حقوقی — " + profile.LegalName
			} else {
				label = "حقیقی — " + profile.FirstName + " " + profile.LastName
			}
			button := qt.NewQPushButton5(fmt.Sprintf("ویرایش #%d — %s", profile.ID, label), p.widget)
			button.OnClicked(func() { loadProfile(profile) })
			profileList.AddWidget(button.QWidget)
			profileButtons = append(profileButtons, button)
		}
	}
	newButton.OnClicked(func() { clearForm(); setType(domain.IssuerTypeIndividual) })
	saveButton.OnClicked(func() {
		for _, setting := range []domain.Setting{{Key: "invoice_brand_name", Value: strings.TrimSpace(brandName.Text())}, {Key: "invoice_logo_path", Value: strings.TrimSpace(logoPath.Text())}} {
			if setting.Key == "invoice_logo_path" && setting.Value != "" {
				if info, err := os.Stat(setting.Value); err != nil || info.IsDir() {
					status.SetText("فایل لوگو در مسیر انتخاب‌شده پیدا نشد.")
					return
				}
			}
			if err := contacts.SetSetting(ctx, setting); err != nil {
				status.SetText("ذخیره برند یا لوگو ناموفق بود: " + err.Error())
				return
			}
		}
		profile := domain.IssuerProfile{ID: editingID, Type: profileType, FirstName: firstName.Text(), LastName: lastName.Text(), LegalName: companyName.Text(), TaxID: taxID.Text(), Address: address.Text(), Email: email.Text()}
		if profileType == domain.IssuerTypeIndividual {
			profile.Name = profile.FirstName + " " + profile.LastName
		} else {
			profile.Name = profile.LegalName
		}
		for _, field := range phoneFields {
			if field.Text() != "" {
				profile.Phones = append(profile.Phones, field.Text())
			}
		}
		for _, row := range accountRows {
			bankLabel := row.bankName.Text()
			if bankLabel == "بانک شناسایی نشده" {
				bankLabel = ""
			}
			account := domain.IssuerBankDetails{BankName: bankLabel, CardNumber: strings.TrimSpace(row.card.Text()), AccountNumber: strings.TrimSpace(row.account.Text()), IBAN: strings.TrimSpace(row.iban.Text())}
			if account.CardNumber != "" {
				result := banking.ValidateCard(account.CardNumber)
				if !result.Valid {
					status.SetText("شماره کارت معتبر نیست: " + result.Code)
					return
				}
				account.CardNumber = result.Normalized
			}
			if account.AccountNumber != "" {
				result := banking.ValidateAccount(account.AccountNumber)
				if !result.Valid {
					status.SetText("شماره حساب معتبر نیست: " + result.Code)
					return
				}
				account.AccountNumber = result.Normalized
			}
			if account.IBAN != "" {
				result := banking.ValidateIBAN(account.IBAN)
				if !result.Valid {
					status.SetText("شماره شبا معتبر نیست: " + result.Code)
					return
				}
				if account.BankName != "" && result.Bank != nil && account.BankName != result.Bank.Name {
					status.SetText("بانک شناسایی‌شده از شبا با بانک کارت هم‌خوانی ندارد.")
					return
				}
				account.IBAN = result.Normalized
			}
			if account.CardNumber != "" || account.AccountNumber != "" || account.IBAN != "" {
				profile.BankDetails = append(profile.BankDetails, account)
			}
		}
		id, err := contacts.SaveIssuerProfile(ctx, profile)
		if err != nil {
			status.SetText("ذخیره انجام نشد: " + err.Error())
			return
		}
		editingID = id
		status.SetText(fmt.Sprintf("پروفایل شماره %d ذخیره شد.", id))
		saveButton.SetText("ذخیره تغییرات")
		refreshProfiles()
	})
	setType(domain.IssuerTypeIndividual)
	refreshProfiles()
	p.layout.AddStretch()
	scroll := qt.NewQScrollArea(parent)
	scroll.SetWidgetResizable(true)
	scroll.SetWidget(p.widget)
	return page{widget: scroll.QWidget, layout: p.layout}
}

func loadInvoiceFootnote(ctx context.Context, contacts *application.Contacts) string {
	setting, err := contacts.Setting(ctx, "invoice_footnote")
	if err != nil {
		return ""
	}
	return setting.Value
}

func loadInvoiceFootnotes(ctx context.Context, contacts *application.Contacts) []string {
	value := loadInvoiceFootnote(ctx, contacts)
	if value == "" {
		return nil
	}
	var items []string
	if json.Unmarshal([]byte(value), &items) == nil {
		return items
	}
	return strings.Split(value, "\n") // migrate the earlier multiline setting without losing its content
}

func loadInvoiceSetting(ctx context.Context, contacts *application.Contacts, key string) string {
	setting, err := contacts.Setting(ctx, key)
	if err != nil {
		return ""
	}
	return setting.Value
}
