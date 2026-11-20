package main

import (
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hesora-ir/hesora-free/internal/application"
	"github.com/hesora-ir/hesora-free/internal/domain"
	qt "github.com/mappu/miqt/qt6"
)

func buildTemplatePage(parent *qt.QWidget, templates *application.Templates, ctx context.Context) page {
	p := newPage(parent, "قالب چاپ", "قالب فاکتور، شماره‌گذاری و عناصر چاپ را تنظیم کنید")
	card, form := newSurface(p.widget, "طراحی قالب فاکتور", "پیش‌نمایش خروجی با دادهٔ نمونه در مرورگر باز می‌شود.")
	p.layout.AddWidget(card)
	selector := qt.NewQComboBox(card)
	selector.AddItem("قالب جدید")
	name := qt.NewQLineEdit(card)
	prefix := qt.NewQLineEdit(card)
	pageSize := qt.NewQComboBox(card)
	pageSize.AddItem("A4")
	pageSize.AddItem("Letter")
	orientation := qt.NewQComboBox(card)
	orientation.AddItem("عمودی")
	orientation.AddItem("افقی")
	font := qt.NewQLineEdit(card)
	font.SetPlaceholderText("IRANSansX")
	fontSize := qt.NewQSpinBox(card)
	fontSize.SetRange(8, 32)
	primary, accent := qt.NewQLineEdit(card), qt.NewQLineEdit(card)
	primary.SetPlaceholderText("#192631")
	accent.SetPlaceholderText("#7058ED")
	showLogo, showTax, showBanks := qt.NewQCheckBox(card), qt.NewQCheckBox(card), qt.NewQCheckBox(card)
	showSeal, showSignature := qt.NewQCheckBox(card), qt.NewQCheckBox(card)
	showLogo.SetText("نمایش لوگو")
	showTax.SetText("نمایش مالیات")
	showBanks.SetText("نمایش اطلاعات بانکی")
	showSeal.SetText("جایگاه مهر")
	showSignature.SetText("جایگاه امضا")
	status := qt.NewQLabel5("", card)
	status.SetWordWrap(true)
	current := domain.DocumentTemplate{Name: "قالب استاندارد هسورا", Prefix: "FACT-", PageSize: "A4", Orientation: "portrait", FontFamily: "IRANSansX", FontSize: 10, PrimaryColor: "#192631", AccentColor: "#7058ED", ShowLogo: true, ShowTax: true, ShowBankAccounts: true}
	logoStatus, sealStatus, signatureStatus := qt.NewQLabel5("تصویری انتخاب نشده", card), qt.NewQLabel5("تصویری انتخاب نشده", card), qt.NewQLabel5("تصویری انتخاب نشده", card)
	for _, item := range []struct {
		title, field string
		status       *qt.QLabel
	}{{"لوگو", "logo", logoStatus}, {"مهر", "seal", sealStatus}, {"امضا", "signature", signatureStatus}} {
		row := qt.NewQHBoxLayout2()
		choose := qt.NewQPushButton5("انتخاب تصویر "+item.title, card)
		remove := qt.NewQPushButton5("حذف", card)
		row.AddWidget(choose.QWidget)
		row.AddWidget(remove.QWidget)
		row.AddWidget(item.status.QWidget)
		form.AddLayout(row.QLayout)
		field, statusLabel := item.field, item.status
		choose.OnClicked(func() {
			path := qt.QFileDialog_GetOpenFileName4(card, "انتخاب تصویر "+item.title, "", "تصویر (*.png *.jpg *.jpeg)")
			if path == "" {
				return
			}
			contents, readErr := os.ReadFile(path)
			if readErr != nil {
				status.SetText("خواندن تصویر ناموفق بود: " + readErr.Error())
				return
			}
			mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
			if mimeType == "image/jpg" {
				mimeType = "image/jpeg"
			}
			asset, imageErr := domain.EncodeTemplateImage(contents, mimeType)
			if imageErr != nil {
				status.SetText("تصویر قالب نامعتبر است: " + imageErr.Error())
				return
			}
			switch field {
			case "logo":
				current.LogoData = asset
			case "seal":
				current.SealData = asset
			case "signature":
				current.SignatureData = asset
			}
			statusLabel.SetText("تصویر انتخاب شد؛ پس از ذخیره در همگام‌سازی می‌رود.")
		})
		remove.OnClicked(func() {
			switch field {
			case "logo":
				current.LogoData = ""
			case "seal":
				current.SealData = ""
			case "signature":
				current.SignatureData = ""
			}
			statusLabel.SetText("تصویر حذف شد؛ برای ثبت، قالب را ذخیره کنید.")
		})
	}
	thankYou := qt.NewQLineEdit(card)
	for _, row := range []struct {
		label  string
		widget *qt.QWidget
	}{
		{"قالب برای ویرایش", selector.QWidget}, {"نام قالب", name.QWidget}, {"پیشوند شماره", prefix.QWidget},
		{"اندازه صفحه", pageSize.QWidget}, {"جهت صفحه", orientation.QWidget}, {"نام فونت فارسی", font.QWidget},
		{"اندازه فونت", fontSize.QWidget}, {"رنگ اصلی (#RRGGBB)", primary.QWidget}, {"رنگ تأکید (#RRGGBB)", accent.QWidget},
		{"گزینه‌های چاپ", showLogo.QWidget}, {"", showTax.QWidget}, {"", showBanks.QWidget}, {"", showSeal.QWidget}, {"", showSignature.QWidget},
		{"متن پایانی", thankYou.QWidget},
	} {
		if row.label != "" {
			form.AddWidget(qt.NewQLabel5(row.label, card).QWidget)
		}
		form.AddWidget(row.widget)
	}
	actions := qt.NewQHBoxLayout2()
	form.AddLayout(actions.QLayout)
	save := qt.NewQPushButton5("ذخیره قالب", card)
	preview := qt.NewQPushButton5("پیش‌نمایش چاپ", card)
	actions.AddWidget(save.QWidget)
	actions.AddWidget(preview.QWidget)
	form.AddWidget(status.QWidget)

	saved, err := templates.DocumentTemplates(ctx)
	if err != nil {
		status.SetText("خواندن قالب‌ها ناموفق بود: " + err.Error())
	}
	for _, item := range saved {
		selector.AddItem(item.Name)
	}
	setFields := func(t domain.DocumentTemplate) {
		current = t
		name.SetText(t.Name)
		prefix.SetText(t.Prefix)
		pageSize.SetCurrentIndex(0)
		if t.PageSize == "Letter" {
			pageSize.SetCurrentIndex(1)
		}
		orientation.SetCurrentIndex(0)
		if t.Orientation == "landscape" {
			orientation.SetCurrentIndex(1)
		}
		font.SetText(t.FontFamily)
		fontSize.SetValue(t.FontSize)
		primary.SetText(t.PrimaryColor)
		accent.SetText(t.AccentColor)
		showLogo.SetChecked(t.ShowLogo)
		showTax.SetChecked(t.ShowTax)
		showBanks.SetChecked(t.ShowBankAccounts)
		showSeal.SetChecked(t.ShowSeal)
		showSignature.SetChecked(t.ShowSignature)
		logoStatus.SetText(assetLabel(t.LogoData))
		sealStatus.SetText(assetLabel(t.SealData))
		signatureStatus.SetText(assetLabel(t.SignatureData))
		thankYou.SetText(t.ThankYou)
	}
	setFields(current)
	if len(saved) > 0 {
		selector.SetCurrentIndex(1)
		setFields(saved[0])
	}
	selector.OnCurrentIndexChanged(func(index int) {
		if index == 0 {
			setFields(domain.DocumentTemplate{Name: "قالب جدید", Prefix: "FACT-", PageSize: "A4", Orientation: "portrait", FontFamily: "IRANSansX", FontSize: 10, PrimaryColor: "#192631", AccentColor: "#7058ED", ShowLogo: true, ShowTax: true, ShowBankAccounts: true})
			return
		}
		if index-1 >= 0 && index-1 < len(saved) {
			setFields(saved[index-1])
		}
	})
	readFields := func() domain.DocumentTemplate {
		t := current
		t.Name, t.Prefix, t.PageSize, t.FontFamily = strings.TrimSpace(name.Text()), strings.TrimSpace(prefix.Text()), pageSize.CurrentText(), strings.TrimSpace(font.Text())
		t.Orientation = "portrait"
		if orientation.CurrentIndex() == 1 {
			t.Orientation = "landscape"
		}
		t.FontSize, t.PrimaryColor, t.AccentColor = fontSize.Value(), strings.TrimSpace(primary.Text()), strings.TrimSpace(accent.Text())
		t.ShowLogo, t.ShowTax, t.ShowBankAccounts = showLogo.IsChecked(), showTax.IsChecked(), showBanks.IsChecked()
		t.ShowSeal, t.ShowSignature, t.ThankYou = showSeal.IsChecked(), showSignature.IsChecked(), strings.TrimSpace(thankYou.Text())
		return t
	}
	save.OnClicked(func() {
		t := readFields()
		id, saveErr := templates.SaveTemplate(ctx, t)
		if saveErr != nil {
			status.SetText("ذخیره قالب ناموفق بود: " + saveErr.Error())
			return
		}
		t.ID = id
		if t.ID == current.ID && t.ID > 0 {
			for i := range saved {
				if saved[i].ID == id {
					saved[i] = t
				}
			}
		} else {
			saved = append(saved, t)
			selector.AddItem(t.Name)
			selector.SetCurrentIndex(len(saved))
		}
		setFields(t)
		status.SetText("قالب ذخیره شد و برای خروجی PDF/HTML قابل استفاده است.")
	})
	preview.OnClicked(func() {
		t := readFields()
		date := time.Now()
		customer, _ := json.Marshal(map[string]string{"name": "مشتری نمونه", "phone": "09120000000", "address": "تهران"})
		issuer, _ := json.Marshal(map[string]string{"name": "فروشگاه نمونه", "phone": "02100000000", "address": "تهران"})
		invoicePrefix := strings.TrimSpace(t.Prefix)
		if invoicePrefix == "" {
			invoicePrefix = "FACT-"
		}
		doc := domain.InvoiceDocument{Number: invoicePrefix + "001", Kind: "invoice", DocumentType: "sales", Status: domain.InvoiceIssued, Currency: domain.CurrencyToman, IssueDate: date, CustomerSnapshot: string(customer), IssuerSnapshot: string(issuer), Items: []domain.InvoiceItem{{Description: "کالای نمونه", Quantity: 1, Unit: "عدد", UnitPrice: 125000}}}
		html, previewErr := templates.PreviewInvoice(ctx, doc, t)
		if previewErr != nil {
			status.SetText("پیش‌نمایش ناموفق بود: " + previewErr.Error())
			return
		}
		path := filepath.Join(os.TempDir(), fmt.Sprintf("hesora-template-preview-%d.html", time.Now().UnixNano()))
		if err := os.WriteFile(path, []byte(html), 0600); err != nil {
			status.SetText("ذخیره پیش‌نمایش ناموفق بود: " + err.Error())
			return
		}
		url := "file:///" + strings.ReplaceAll(filepath.ToSlash(path), " ", "%20")
		if !qt.QDesktopServices_OpenUrl(qt.NewQUrl3(url)) {
			status.SetText("بازکردن مرورگر برای پیش‌نمایش ناموفق بود.")
			return
		}
		status.SetText("پیش‌نمایش با تنظیمات فعلی باز شد.")
	})
	scroll := qt.NewQScrollArea(parent)
	scroll.SetWidgetResizable(true)
	scroll.SetVerticalScrollBarPolicy(qt.ScrollBarAsNeeded)
	scroll.SetWidget(p.widget)
	return page{widget: scroll.QWidget, layout: p.layout}
}

func assetLabel(data string) string {
	if data == "" {
		return "تصویری انتخاب نشده"
	}
	return "تصویر پیوست شده"
}
