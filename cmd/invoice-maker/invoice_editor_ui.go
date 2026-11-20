package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hesora-ir/hesora-free/internal/application"
	"github.com/hesora-ir/hesora-free/internal/domain"
	qt "github.com/mappu/miqt/qt6"
)

type invoiceEditorOptions struct {
	templates *application.Templates
	contacts  *application.Contacts
	onIssued  func()
}

type invoiceChargeEditor struct {
	widget       *qt.QWidget
	name, amount *qt.QLineEdit
}

func buildInvoicePage(parent *qt.QWidget, contacts *application.Contacts, invoices *application.Invoices, ctx context.Context, existing *domain.InvoiceDocument, options ...invoiceEditorOptions) page {
	var opts invoiceEditorOptions
	if len(options) > 0 {
		opts = options[0]
	}
	opts.contacts = contacts
	p := newPage(parent, "صدور فاکتور", "از انتخاب مشتری تا صدور سند؛ همه مبالغ با واحد پول مشخص")
	currency := domain.CurrencyToman
	if setting, err := contacts.Setting(ctx, "default_currency"); err == nil && setting.Value == "IRR" {
		currency = domain.CurrencyRial
	}
	if existing != nil {
		currency = existing.Currency
	}
	currencyName := "تومان"
	if currency == domain.CurrencyRial {
		currencyName = "ریال"
	}

	identity, identityLayout := newSurface(p.widget, "۱. مشخصات سند", "اطلاعات مشتری و صادرکننده همراه سند ذخیره می‌شود.")
	p.layout.AddWidget(identity)
	kind := qt.NewQComboBox(identity)
	kind.AddItem("پیش‌فاکتور — اعلام قیمت")
	kind.AddItem("فاکتور — ایجاد بدهی پس از صدور")
	documentType := qt.NewQComboBox(identity)
	typeLabels := []string{"فاکتور فروش", "فاکتور اعتباری", "فاکتور خدمات", "اعلام قیمت", "فاکتور اصلاحی", "پیش‌فاکتور"}
	for _, label := range typeLabels {
		documentType.AddItem(label)
	}
	metadata := qt.NewQHBoxLayout2()
	labeledInvoiceField(metadata, identity, "نوع سند", kind.QWidget)
	labeledInvoiceField(metadata, identity, "دستهٔ فاکتور", documentType.QWidget)
	currencyLabel := qt.NewQLabel5(currencyName, identity)
	labeledInvoiceField(metadata, identity, "واحد همه مبالغ این سند", currencyLabel.QWidget)
	identityLayout.AddLayout(metadata.QLayout)
	parties := qt.NewQHBoxLayout2()
	customerCombo, issuerCombo := qt.NewQComboBox(identity), qt.NewQComboBox(identity)
	labeledInvoiceField(parties, identity, "مشتری", customerCombo.QWidget)
	labeledInvoiceField(parties, identity, "صادرکننده", issuerCombo.QWidget)
	identityLayout.AddLayout(parties.QLayout)
	reload := qt.NewQPushButton5("به‌روزرسانی مشتریان و صادرکنندگان", identity)
	setInvoiceObjectName(reload, "secondaryAction")
	identityLayout.AddWidget(reload.QWidget)
	partyStatus := qt.NewQLabel5("", identity)
	partyStatus.SetWordWrap(true)
	setInvoiceObjectName(partyStatus, "muted")
	identityLayout.AddWidget(partyStatus.QWidget)
	if existing != nil {
		if existing.Kind == "invoice" {
			kind.SetCurrentIndex(1)
		}
		kind.SetEnabled(false)
		typeMap := map[string]int{"sales": 0, "credit": 1, "service": 2, "quotation": 3, "corrective": 4, "proforma": 5}
		if index, ok := typeMap[existing.DocumentType]; ok {
			documentType.SetCurrentIndex(index)
		} else if existing.Kind == "proforma" {
			documentType.SetCurrentIndex(5)
		}
	}

	itemsPanel, itemsPanelLayout := newSurface(p.widget, "۲. اقلام فاکتور", "شرح، تعداد و قیمت واحد را وارد کنید. مبالغ به "+currencyName+" هستند.")
	p.layout.AddWidget(itemsPanel)
	itemsLayout := qt.NewQVBoxLayout2()
	itemsLayout.SetSpacing(12)
	itemsPanelLayout.AddLayout(itemsLayout.QLayout)
	addItemButton := qt.NewQPushButton5("افزودن قلم", itemsPanel)
	setInvoiceObjectName(addItemButton, "secondaryAction")
	itemsPanelLayout.AddWidget(addItemButton.QWidget)

	finance, financeLayout := newSurface(p.widget, "۳. جمع مبالغ", "تخفیف پیش از مالیات از مبلغ اقلام کسر می‌شود.")
	p.layout.AddWidget(finance)
	financeRow := qt.NewQHBoxLayout2()
	discountInput, taxInput := qt.NewQLineEdit(finance), qt.NewQLineEdit(finance)
	discountModeBox := qt.NewQComboBox(finance)
	discountModeBox.AddItem("مبلغ ثابت")
	discountModeBox.AddItem("درصد")
	discountInput.SetPlaceholderText("۰")
	taxInput.SetPlaceholderText("مثلاً ۹ یا ۹٫۵")
	discountMode := domain.DiscountFixed
	if existing != nil && existing.InvoiceDiscount.Mode == domain.DiscountPercent {
		discountMode = domain.DiscountPercent
	}
	if discountMode == domain.DiscountPercent {
		discountModeBox.SetCurrentIndex(1)
	}
	labeledInvoiceField(financeRow, finance, "نوع تخفیف", discountModeBox.QWidget)
	labeledInvoiceField(financeRow, finance, "میزان تخفیف", discountInput.QWidget)
	labeledInvoiceField(financeRow, finance, "مالیات (درصد)", taxInput.QWidget)
	financeLayout.AddLayout(financeRow.QLayout)
	breakdown := qt.NewQLabel5("", finance)
	breakdown.SetWordWrap(true)
	setInvoiceObjectName(breakdown, "muted")
	financeLayout.AddWidget(breakdown.QWidget)
	totals := qt.NewQLabel5("—", finance)
	totals.SetWordWrap(true)
	setInvoiceObjectName(totals, "grandTotal")
	financeLayout.AddWidget(totals.QWidget)
	if existing != nil {
		discountText := strconv.FormatInt(int64(existing.InvoiceDiscount.Value), 10)
		if discountMode == domain.DiscountPercent {
			discountText = formatInvoicePercent(int64(existing.InvoiceDiscount.Value))
		}
		discountInput.SetText(discountText)
		taxInput.SetText(formatInvoicePercent(existing.TaxRateBPS))
	}
	var updateTotals func()
	chargeRows := make([]invoiceChargeEditor, 0)
	chargePanel, chargeLayout := newSurface(p.widget, "هزینه‌های اضافه", "مانند ارسال؛ جدا از جمع مبلغ اقلام محاسبه و پس از جمع کل نمایش داده می‌شوند.")
	p.layout.AddWidget(chargePanel)
	chargeRowsLayout := qt.NewQVBoxLayout2()
	chargeLayout.AddLayout(chargeRowsLayout.QLayout)
	addCharge := func(charge domain.AdditionalCharge) {
		widget := qt.NewQWidget(chargePanel)
		row := qt.NewQHBoxLayout2()
		name, amount := qt.NewQLineEdit(widget), qt.NewQLineEdit(widget)
		name.SetPlaceholderText("شرح هزینه (مثلاً ارسال)")
		name.SetText(charge.Description)
		amount.SetPlaceholderText("مبلغ اضافه")
		if charge.Amount > 0 {
			amount.SetText(strconv.FormatInt(int64(charge.Amount), 10))
		}
		remove := qt.NewQPushButton5("حذف", widget)
		setInvoiceObjectName(remove, "removeItem")
		row.AddWidget(name.QWidget)
		row.AddWidget(amount.QWidget)
		row.AddWidget(remove.QWidget)
		widget.SetLayout(row.QLayout)
		chargeRowsLayout.AddWidget(widget)
		chargeRows = append(chargeRows, invoiceChargeEditor{widget: widget, name: name, amount: amount})
		remove.OnClicked(func() {
			for i, candidate := range chargeRows {
				if candidate.widget == widget {
					chargeRows = append(chargeRows[:i], chargeRows[i+1:]...)
					break
				}
			}
			chargeRowsLayout.RemoveWidget(widget)
			widget.DeleteLater()
			updateTotals()
		})
	}
	if existing != nil {
		for _, charge := range existing.AdditionalCharges {
			addCharge(charge)
		}
	}
	chargeAddButton := qt.NewQPushButton5("+ افزودن هزینه", chargePanel)
	chargeLayout.AddWidget(chargeAddButton.QWidget)

	bankPanel, bankPanelLayout := newSurface(p.widget, "۴. اطلاعات پرداخت", "حساب‌های انتخاب‌شده در سند نمایش داده می‌شوند.")
	p.layout.AddWidget(bankPanel)
	bankHint := qt.NewQLabel5("", bankPanel)
	bankHint.SetWordWrap(true)
	setInvoiceObjectName(bankHint, "muted")
	bankPanelLayout.AddWidget(bankHint.QWidget)
	bankLayout := qt.NewQVBoxLayout2()
	bankPanelLayout.AddLayout(bankLayout.QLayout)
	var customers []domain.Customer
	var issuers []domain.IssuerProfile
	var pendingCustomerID int64
	var bankChecks []*qt.QCheckBox
	rebuildBanks := func() {
		for _, check := range bankChecks {
			bankLayout.RemoveWidget(check.QWidget)
			check.DeleteLater()
		}
		bankChecks = nil
		index := issuerCombo.CurrentIndex()
		bankHint.SetText("حسابی ثبت نشده است؛ از بخش اطلاعات پایه حساب بانکی اضافه کنید.")
		if index < 0 || index >= len(issuers) {
			return
		}
		for _, bank := range issuers[index].BankDetails {
			check := qt.NewQCheckBox(bankPanel)
			check.SetText(fmt.Sprintf("کارت: %s  |  حساب: %s  |  شبا: %s  |  بانک: %s", bank.CardNumber, bank.AccountNumber, bank.IBAN, bank.BankName))
			check.SetChecked(true)
			bankLayout.AddWidget(check.QWidget)
			bankChecks = append(bankChecks, check)
		}
		if len(bankChecks) > 0 {
			bankHint.SetText("برای حذف یک حساب از این سند، تیک آن را بردارید.")
		}
	}
	issuerCombo.OnCurrentIndexChanged(func(int) { rebuildBanks() })

	notePanel, noteLayout := newSurface(p.widget, "۵. شرایط و توضیحات", "شرایط اعتبار، تحویل، ضمانت و سایر توضیحات این سند")
	p.layout.AddWidget(notePanel)
	note := qt.NewQPlainTextEdit(notePanel)
	note.SetPlaceholderText("توضیحات فاکتور (اختیاری)")
	note.SetMinimumHeight(100)
	note.SetMaximumHeight(150)
	if existing != nil {
		note.SetPlainText(existing.Note)
	}
	noteLayout.AddWidget(note.QWidget)
	footnoteChecks := make([]*qt.QCheckBox, 0)
	for _, text := range loadInvoiceFootnotes(ctx, contacts) {
		check := qt.NewQCheckBox(notePanel)
		check.SetText(text)
		noteLayout.AddWidget(check.QWidget)
		footnoteChecks = append(footnoteChecks, check)
	}
	status := qt.NewQLabel5("پیش‌فاکتور و پیش‌نویس بدهی ایجاد نمی‌کنند؛ بدهی با صدور فاکتور نهایی ثبت می‌شود.", p.widget)
	status.SetWordWrap(true)
	p.layout.AddWidget(status.QWidget)
	actions := qt.NewQHBoxLayout2()
	saveButton := qt.NewQPushButton5("ذخیره پیش‌فاکتور", p.widget)
	setInvoiceObjectName(saveButton, "secondaryAction")
	issueButton := qt.NewQPushButton5("صدور فاکتور نهایی", p.widget)
	if existing != nil && existing.Status == domain.InvoiceIssued {
		issueButton.SetEnabled(false)
		issueButton.SetText("ویرایش فاکتور صادرشده")
	} else if documentType.CurrentIndex() == 3 {
		issueButton.SetEnabled(false)
		issueButton.SetText("اعلام قیمت بدون ایجاد بدهی")
	}
	newButton := qt.NewQPushButton5("سند جدید", p.widget)
	setInvoiceObjectName(newButton, "secondaryAction")
	actions.AddWidget(saveButton.QWidget)
	actions.AddWidget(issueButton.QWidget)
	actions.AddWidget(newButton.QWidget)
	p.layout.AddLayout(actions.QLayout)
	p.layout.AddStretch()

	itemEditors := make([]*invoiceItemEditor, 0)
	updateTotals = func() {}
	chargeAddButton.OnClicked(func() {
		addCharge(domain.AdditionalCharge{})
		row := chargeRows[len(chargeRows)-1]
		row.name.OnTextChanged(func(string) { updateTotals() })
		row.amount.OnTextChanged(func(string) { updateTotals() })
		updateTotals()
	})
	discountModeBox.OnCurrentIndexChanged(func(index int) {
		if index == 1 {
			discountMode = domain.DiscountPercent
			discountInput.SetPlaceholderText("مثلاً ۱۰٪")
		} else {
			discountMode = domain.DiscountFixed
			discountInput.SetPlaceholderText("مبلغ به " + currencyName)
		}
		updateTotals()
	})
	addItem := func(item domain.InvoiceItem) {
		row := qt.NewQWidget(itemsPanel)
		setInvoiceObjectName(row, "itemRow")
		layout := qt.NewQHBoxLayout2()
		layout.SetContentsMargins(0, 0, 0, 0)
		layout.SetSpacing(10)
		row.SetLayout(layout.QLayout)
		name, quantity, unit, price := qt.NewQLineEdit(row), qt.NewQLineEdit(row), qt.NewQLineEdit(row), qt.NewQLineEdit(row)
		name.SetMinimumWidth(180)
		name.SetPlaceholderText("نام کالا یا شرح خدمت")
		name.SetText(item.Description)
		quantity.SetMaximumWidth(85)
		quantity.SetText(strconv.FormatInt(item.Quantity, 10))
		unit.SetMaximumWidth(95)
		unit.SetPlaceholderText("عدد، ساعت…")
		unit.SetText(item.Unit)
		price.SetMaximumWidth(160)
		price.SetPlaceholderText("۰")
		if item.UnitPrice != 0 {
			price.SetText(strconv.FormatInt(int64(item.UnitPrice), 10))
		}
		total := qt.NewQLabel5("—", row)
		total.SetMinimumWidth(110)
		setInvoiceObjectName(total, "sectionTitle")
		remove := qt.NewQPushButton5("حذف", row)
		setInvoiceObjectName(remove, "removeItem")
		labeledInvoiceField(layout, row, "شرح کالا یا خدمت", name.QWidget)
		labeledInvoiceField(layout, row, "تعداد", quantity.QWidget)
		labeledInvoiceField(layout, row, "واحد", unit.QWidget)
		labeledInvoiceField(layout, row, "قیمت واحد", price.QWidget)
		labeledInvoiceField(layout, row, "مبلغ ردیف", total.QWidget)
		layout.AddWidget(remove.QWidget)
		editor := &invoiceItemEditor{row: row, name: name, quantity: quantity, unit: unit, price: price, remove: remove, total: total, discount: item.Discount}
		itemEditors = append(itemEditors, editor)
		itemsLayout.AddWidget(row)
		model := qt.NewQStringListModel3(row.QObject)
		completer := qt.NewQCompleter5(model.QAbstractItemModel, row.QObject)
		completer.SetFilterMode(qt.MatchContains)
		name.SetCompleter(completer)
		suggestions := map[string]domain.InvoiceItem{}
		name.OnTextEdited(func(text string) {
			items, err := invoices.SuggestedItems(ctx, text, currency)
			if err != nil {
				model.SetStringList(nil)
				return
			}
			labels := make([]string, 0, len(items))
			suggestions = map[string]domain.InvoiceItem{}
			for _, candidate := range items {
				label := fmt.Sprintf("%s ? %s %s", candidate.Description, formatInvoiceMoney(int64(candidate.UnitPrice)), currencyName)
				labels = append(labels, label)
				suggestions[label] = candidate
			}
			model.SetStringList(labels)
		})
		completer.OnActivated(func(label string) {
			if candidate, ok := suggestions[label]; ok {
				name.SetText(candidate.Description)
				unit.SetText(candidate.Unit)
				price.SetText(strconv.FormatInt(int64(candidate.UnitPrice), 10))
				updateTotals()
			}
		})
		name.OnTextChanged(func(string) { updateTotals() })
		quantity.OnTextChanged(func(string) { updateTotals() })
		price.OnTextChanged(func(string) { updateTotals() })
		remove.OnClicked(func() {
			if len(itemEditors) == 1 {
				status.SetText("سند باید دست‌کم یک قلم داشته باشد.")
				return
			}
			for i, candidate := range itemEditors {
				if candidate == editor {
					itemEditors = append(itemEditors[:i], itemEditors[i+1:]...)
					break
				}
			}
			itemsLayout.RemoveWidget(row)
			row.DeleteLater()
			updateTotals()
		})
	}
	if existing != nil && len(existing.Items) > 0 {
		for _, item := range existing.Items {
			addItem(item)
		}
	} else {
		addItem(domain.InvoiceItem{Quantity: 1})
	}
	addItemButton.OnClicked(func() { addItem(domain.InvoiceItem{Quantity: 1}); updateTotals() })
	readFinancials := func() ([]domain.InvoiceItem, domain.Discount, int64, error) {
		var items []domain.InvoiceItem
		for i, editor := range itemEditors {
			qty, err := parseInvoiceAmount(editor.quantity.Text())
			if err != nil || qty == 0 {
				return nil, domain.Discount{}, 0, fmt.Errorf("تعداد ردیف %d باید عدد صحیح مثبت باشد", i+1)
			}
			price, err := parseInvoiceAmount(editor.price.Text())
			if err != nil {
				return nil, domain.Discount{}, 0, fmt.Errorf("قیمت ردیف %d نامعتبر است", i+1)
			}
			item := domain.InvoiceItem{Description: strings.TrimSpace(editor.name.Text()), Quantity: qty, Unit: strings.TrimSpace(editor.unit.Text()), UnitPrice: domain.Money(price), Discount: editor.discount}
			items = append(items, item)
		}
		discount, err := parseInvoiceAmount(discountInput.Text())
		if discountMode == domain.DiscountPercent {
			discount, err = parseInvoicePercent(discountInput.Text())
		}
		if err != nil {
			return nil, domain.Discount{}, 0, fmt.Errorf("تخفیف نامعتبر است")
		}
		tax, err := parseInvoicePercent(taxInput.Text())
		if err != nil {
			return nil, domain.Discount{}, 0, fmt.Errorf("مالیات باید درصدی بین صفر تا ۱۰۰ باشد")
		}
		return items, domain.Discount{Mode: discountMode, Value: domain.Money(discount)}, tax, nil
	}
	readCharges := func() ([]domain.AdditionalCharge, error) {
		charges := make([]domain.AdditionalCharge, 0, len(chargeRows))
		for i, row := range chargeRows {
			name, raw := strings.TrimSpace(row.name.Text()), strings.TrimSpace(row.amount.Text())
			if name == "" && raw == "" {
				continue
			}
			if name == "" {
				return nil, fmt.Errorf("شرح هزینهٔ اضافهٔ ردیف %d را وارد کنید", i+1)
			}
			amount, err := parseInvoiceAmount(raw)
			if err != nil || amount < 0 {
				return nil, fmt.Errorf("مبلغ هزینهٔ اضافهٔ ردیف %d نامعتبر است", i+1)
			}
			charges = append(charges, domain.AdditionalCharge{Description: name, Amount: domain.Money(amount)})
		}
		return charges, nil
	}
	updateTotals = func() {
		for _, editor := range itemEditors {
			editor.total.SetText("—")
			qty, qErr := parseInvoiceAmount(editor.quantity.Text())
			price, pErr := parseInvoiceAmount(editor.price.Text())
			if qErr == nil && pErr == nil && qty > 0 {
				item := domain.InvoiceItem{Description: strings.TrimSpace(editor.name.Text()), Quantity: qty, UnitPrice: domain.Money(price), Discount: editor.discount}
				if amount, err := item.LineTotal(); err == nil {
					editor.total.SetText(formatInvoiceMoney(int64(amount)))
				}
			}
		}
		items, discount, tax, err := readFinancials()
		if err == nil {
			charges, chargeErr := readCharges()
			if chargeErr != nil {
				err = chargeErr
			} else {
				var calculated domain.FinancialTotals
				calculated, err = domain.CalculateFinancialsWithCharges(items, discount, tax, charges)
				if err == nil {
					breakdown.SetText(fmt.Sprintf("جمع تعداد: %d  •  جمع اقلام: %s  •  تخفیف: %s  •  مالیات: %s  •  هزینه‌های اضافه: %s", calculated.TotalQuantity, formatInvoiceMoney(int64(calculated.Subtotal)), formatInvoiceMoney(int64(calculated.ItemDiscount+calculated.InvoiceDiscount)), formatInvoiceMoney(int64(calculated.Tax)), formatInvoiceMoney(int64(calculated.AdditionalCharges))))
					totals.SetText("جمع قابل پرداخت: " + formatInvoiceMoney(int64(calculated.GrandTotal)) + " " + currencyName + "\n" + domain.AmountInWordsFA(calculated.GrandTotal, currency))
					return
				}
			}
		}
		breakdown.SetText(err.Error())
		totals.SetText("اطلاعات اقلام و مبالغ را تکمیل کنید")
	}

	for _, row := range chargeRows {
		row.name.OnTextChanged(func(string) { updateTotals() })
		row.amount.OnTextChanged(func(string) { updateTotals() })
	}
	documentType.OnCurrentIndexChanged(func(index int) {
		if existing == nil || existing.Status != domain.InvoiceIssued {
			issueButton.SetEnabled(index != 3)
		}
		updateTotals()
	})
	loadParties := func() {
		var customerID, issuerID int64
		if i := customerCombo.CurrentIndex(); i >= 0 && i < len(customers) {
			customerID = customers[i].ID
		}
		if i := issuerCombo.CurrentIndex(); i >= 0 && i < len(issuers) {
			issuerID = issuers[i].ID
		}
		if existing != nil {
			customerID, issuerID = existing.CustomerID, existing.IssuerProfileID
		}
		newCustomers, customerErr := contacts.SearchCustomers(ctx, "")
		newIssuers, issuerErr := contacts.IssuerProfiles(ctx)
		if customerErr != nil || issuerErr != nil {
			partyStatus.SetText("خواندن مشتریان یا صادرکنندگان ناموفق بود؛ دوباره به‌روزرسانی کنید.")
			return
		}
		customerCombo.BlockSignals(true)
		issuerCombo.BlockSignals(true)
		customerCombo.Clear()
		issuerCombo.Clear()
		customers, issuers = newCustomers, newIssuers
		for i, customer := range customers {
			customerCombo.AddItem(customer.Name)
			if customer.ID == customerID {
				customerCombo.SetCurrentIndex(i)
			}
		}
		customerCombo.AddItem("+ ساخت مشتری")
		if pendingCustomerID != 0 {
			for i, customer := range customers {
				if customer.ID == pendingCustomerID {
					customerCombo.SetCurrentIndex(i)
					break
				}
			}
			pendingCustomerID = 0
		}
		for i, issuer := range issuers {
			label := issuer.Name
			if issuer.Type == domain.IssuerTypeCompany {
				label = issuer.LegalName
			}
			if issuer.Type == domain.IssuerTypeIndividual {
				label = strings.TrimSpace(issuer.FirstName + " " + issuer.LastName)
			}
			issuerCombo.AddItem(label)
			if issuer.ID == issuerID {
				issuerCombo.SetCurrentIndex(i)
			}
		}
		customerCombo.BlockSignals(false)
		issuerCombo.BlockSignals(false)
		rebuildBanks()
		partyStatus.SetText("پیش‌فاکتور برای اعلام قیمت است؛ با صدور فاکتور نهایی بدهی ایجاد می‌شود.")
		if len(customers) == 0 || len(issuers) == 0 {
			partyStatus.SetText("ابتدا در بخش مشتریان و اطلاعات پایه، مشتری و صادرکننده ثبت کنید؛ سپس فهرست‌ها را به‌روزرسانی کنید.")
		}
	}
	reload.OnClicked(loadParties)
	loadParties()
	customerCombo.OnCurrentIndexChanged(func(index int) {
		if index != len(customers) {
			return
		}
		openCustomerForm(p.widget, contacts, ctx, nil, func(customer domain.Customer) {
			pendingCustomerID = customer.ID
			loadParties()
			partyStatus.SetText("مشتری ساخته شد و برای این فاکتور انتخاب شد.")
		})
	})
	if existing != nil && existing.BankAccountsSnapshot != "" {
		var selected []domain.IssuerBankDetails
		index := issuerCombo.CurrentIndex()
		if json.Unmarshal([]byte(existing.BankAccountsSnapshot), &selected) == nil && index >= 0 && index < len(issuers) {
			for i, check := range bankChecks {
				candidate := issuers[index].BankDetails[i]
				match := false
				for _, chosen := range selected {
					if candidate.CardNumber == chosen.CardNumber && candidate.AccountNumber == chosen.AccountNumber && candidate.IBAN == chosen.IBAN {
						match = true
						break
					}
				}
				check.SetChecked(match)
			}
		}
	}
	buildDocument := func(documentKind string) (domain.InvoiceDocument, error) {
		var doc domain.InvoiceDocument
		ci, ii := customerCombo.CurrentIndex(), issuerCombo.CurrentIndex()
		if ci < 0 || ci >= len(customers) {
			return doc, fmt.Errorf("یک مشتری ثبت‌شده انتخاب کنید")
		}
		if ii < 0 || ii >= len(issuers) {
			return doc, fmt.Errorf("یک صادرکننده ثبت‌شده انتخاب کنید")
		}
		prefix := "INV-"
		if documentKind == "proforma" {
			prefix = "PF-"
		}
		if existing == nil && opts.templates != nil {
			savedTemplates, templateErr := opts.templates.DocumentTemplates(ctx)
			if templateErr != nil {
				return doc, fmt.Errorf("خواندن پیشوند قالب فاکتور: %w", templateErr)
			}
			if len(savedTemplates) > 0 && strings.TrimSpace(savedTemplates[0].Prefix) != "" {
				prefix = savedTemplates[0].Prefix
			}
		}
		selectedNotes := make([]string, 0)
		for _, check := range footnoteChecks {
			if check.IsChecked() {
				selectedNotes = append(selectedNotes, check.Text())
			}
		}
		finalNote := strings.TrimSpace(note.ToPlainText())
		if len(selectedNotes) > 0 {
			if finalNote != "" {
				finalNote += "\n"
			}
			finalNote += strings.Join(selectedNotes, "\n")
		}
		doc = domain.InvoiceDocument{Kind: documentKind, Status: domain.InvoiceDraft, Currency: currency, CustomerID: customers[ci].ID, IssuerProfileID: issuers[ii].ID, IssueDate: time.Now(), Note: finalNote, Numbering: domain.NumberingConfig{Series: documentKind, Prefix: prefix, StartAt: 1, Width: 6}}
		if existing != nil {
			doc.ID, doc.Number, doc.Kind, doc.Status, doc.Numbering, doc.IssueDate, doc.DueDate = existing.ID, existing.Number, existing.Kind, existing.Status, existing.Numbering, existing.IssueDate, existing.DueDate
		}
		doc.DocumentType = []string{"sales", "credit", "service", "quotation", "corrective", "proforma"}[documentType.CurrentIndex()]
		customer, issuer := customers[ci], issuers[ii]
		phone, address := customer.Phone, customer.Address
		if len(customer.Phones) > 0 {
			phone = customer.Phones[0]
		}
		if len(customer.Addresses) > 0 {
			address = customer.Addresses[0]
		}
		customerJSON, _ := json.Marshal(invoicePartySnapshot{Name: customer.Name, LegalName: customer.LegalName, Phone: phone, Email: customer.Email, Address: address, TaxID: customer.TaxID})
		issuerName, issuerPhone := issuer.Name, issuer.Phone
		if issuer.Type == domain.IssuerTypeIndividual {
			issuerName = strings.TrimSpace(issuer.FirstName + " " + issuer.LastName)
		}
		if len(issuer.Phones) > 0 {
			issuerPhone = issuer.Phones[0]
		}
		issuerJSON, _ := json.Marshal(invoicePartySnapshot{Name: issuerName, LegalName: issuer.LegalName, Phone: issuerPhone, Email: issuer.Email, Address: issuer.Address, TaxID: issuer.TaxID})
		doc.CustomerSnapshot, doc.IssuerSnapshot = string(customerJSON), string(issuerJSON)
		var err error
		doc.Items, doc.InvoiceDiscount, doc.TaxRateBPS, err = readFinancials()
		if err != nil {
			return doc, err
		}
		doc.AdditionalCharges, err = readCharges()
		if err != nil {
			return doc, err
		}
		selected := make([]domain.IssuerBankDetails, 0)
		for i, check := range bankChecks {
			if check.IsChecked() && i < len(issuer.BankDetails) {
				selected = append(selected, issuer.BankDetails[i])
			}
		}
		bankJSON, _ := json.Marshal(selected)
		doc.BankAccountsSnapshot = string(bankJSON)
		return doc, doc.Validate()
	}
	save := func(issue bool) {
		if issue && documentType.CurrentIndex() == 3 {
			status.SetText("اعلام قیمت باید به‌عنوان پیش‌فاکتور ذخیره شود و بدهی ایجاد نمی‌کند.")
			return
		}
		documentKind := "proforma"
		if issue || kind.CurrentIndex() == 1 {
			documentKind = "invoice"
		}
		if documentType.CurrentIndex() == 3 || documentType.CurrentIndex() == 5 {
			documentKind = "proforma"
		}
		if existing != nil {
			documentKind = existing.Kind
		}
		doc, err := buildDocument(documentKind)
		if err != nil {
			status.SetText("اطلاعات سند معتبر نیست: " + err.Error())
			return
		}
		if existing == nil {
			doc.ID, doc.Number, err = invoices.CreateDraft(ctx, doc)
		} else {
			err = invoices.UpdateDraft(ctx, doc)
		}
		if err != nil {
			status.SetText("ذخیره سند انجام نشد: " + err.Error())
			return
		}
		existing = &doc
		kind.SetEnabled(false)
		if issue {
			if doc.Kind == "proforma" {
				err = invoices.ConvertProforma(ctx, doc.ID)
			} else {
				err = invoices.Issue(ctx, doc.ID)
			}
			if err != nil {
				status.SetText("سند " + doc.Number + " ذخیره شد ولی صدور ناموفق بود: " + err.Error())
				return
			}
			saveButton.SetEnabled(false)
			issueButton.SetEnabled(false)
			status.SetText("فاکتور " + doc.Number + " صادر شد.")
			phone := ""
			var customerSnapshot invoicePartySnapshot
			if json.Unmarshal([]byte(doc.CustomerSnapshot), &customerSnapshot) == nil {
				phone = customerSnapshot.Phone
			}
			showIssuedInvoiceActions(parent, doc, phone, ctx, opts.contacts, invoices, opts.templates, opts.onIssued)
		} else {
			if doc.Status == domain.InvoiceIssued {
				status.SetText("فاکتور تسویه‌نشده " + doc.Number + " ویرایش و ذخیره شد.")
			} else {
				status.SetText("سند " + doc.Number + " ذخیره شد؛ هنوز بدهی مشتری ایجاد نشده است.")
			}
		}
	}
	updateActionLabels := func() {
		if kind.CurrentIndex() == 1 {
			saveButton.SetText("ذخیره پیش‌نویس فاکتور")
		} else {
			saveButton.SetText("ذخیره پیش‌فاکتور")
		}
		if existing != nil && existing.Kind == "proforma" {
			issueButton.SetText("ذخیره و تبدیل به فاکتور")
		}
	}
	saveButton.OnClicked(func() { save(false); updateActionLabels() })
	issueButton.OnClicked(func() { save(true) })
	kind.OnCurrentIndexChanged(func(int) { updateActionLabels() })
	newButton.OnClicked(func() {
		existing = nil
		for _, editor := range itemEditors {
			itemsLayout.RemoveWidget(editor.row)
			editor.row.DeleteLater()
		}
		itemEditors = nil
		for _, charge := range chargeRows {
			chargeRowsLayout.RemoveWidget(charge.widget)
			charge.widget.DeleteLater()
		}
		chargeRows = nil
		addItem(domain.InvoiceItem{Quantity: 1})
		discountInput.Clear()
		discountMode = domain.DiscountFixed
		discountModeBox.SetCurrentIndex(0)
		taxInput.Clear()
		note.SetPlainText("")
		if setting, err := contacts.Setting(ctx, "invoice_footnote"); err == nil {
			note.SetPlainText(setting.Value)
		}
		kind.SetEnabled(true)
		saveButton.SetEnabled(true)
		issueButton.SetEnabled(documentType.CurrentIndex() != 3)
		issueButton.SetText("صدور فاکتور نهایی")
		loadParties()
		updateActionLabels()
		updateTotals()
		status.SetText("سند جدید آماده است؛ اطلاعات را وارد کنید.")
	})
	discountInput.OnTextChanged(func(string) { updateTotals() })
	taxInput.OnTextChanged(func(string) { updateTotals() })
	updateActionLabels()
	updateTotals()
	scroll := qt.NewQScrollArea(parent)
	scroll.SetWidgetResizable(true)
	scroll.SetWidget(p.widget)
	return page{widget: scroll.QWidget, layout: p.layout}
}

func parseInvoiceAmount(raw string) (int64, error) {
	raw = normalizeInvoiceDigits(strings.TrimSpace(strings.NewReplacer(",", "", "٬", "").Replace(raw)))
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("عدد صحیح نامنفی وارد کنید")
	}
	return value, nil
}

// Parse percentages into basis points without floating point or silent rounding.
func parseInvoicePercent(raw string) (int64, error) {
	raw = normalizeInvoiceDigits(strings.TrimSpace(strings.ReplaceAll(raw, "٫", ".")))
	if raw == "" {
		return 0, nil
	}
	parts := strings.Split(raw, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, fmt.Errorf("درصد نامعتبر است")
	}
	for _, part := range parts {
		for _, r := range part {
			if r < '0' || r > '9' {
				return 0, fmt.Errorf("درصد نامعتبر است")
			}
		}
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole > 100 {
		return 0, fmt.Errorf("درصد باید بین صفر تا ۱۰۰ باشد")
	}
	fraction := int64(0)
	if len(parts) == 2 {
		if len(parts[1]) == 0 || len(parts[1]) > 2 {
			return 0, fmt.Errorf("حداکثر دو رقم اعشار وارد کنید")
		}
		fraction, _ = strconv.ParseInt(parts[1]+strings.Repeat("0", 2-len(parts[1])), 10, 64)
	}
	bps := whole*100 + fraction
	if bps > 10000 {
		return 0, fmt.Errorf("درصد باید بین صفر تا ۱۰۰ باشد")
	}
	return bps, nil
}

func formatInvoicePercent(bps int64) string {
	return fmt.Sprintf("%d.%02d", bps/100, bps%100)
}

func formatInvoiceMoney(value int64) string {
	raw := formatCustomerMoney(value)
	digits := []rune(raw)
	var out strings.Builder
	for i, digit := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 && digit != '-' && digits[i-1] != '-' {
			out.WriteRune('٬')
		}
		out.WriteRune(digit)
	}
	return out.String()
}
