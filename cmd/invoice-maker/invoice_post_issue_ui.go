package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/hesora-ir/hesora-free/internal/application"
	"github.com/hesora-ir/hesora-free/internal/domain"
	qt "github.com/mappu/miqt/qt6"
	qtpdf "github.com/mappu/miqt/qt6/pdf"
)

func showIssuedInvoiceActions(parent *qt.QWidget, invoice domain.InvoiceDocument, phone string, ctx context.Context, contacts *application.Contacts, invoices *application.Invoices, templates *application.Templates, onReturn func()) {
	window := qt.NewQMainWindow2()
	window.QWidget.SetWindowTitle("فاکتور صادر شد")
	window.QWidget.SetMinimumSize2(520, 250)
	container := qt.NewQWidget(window.QWidget)
	layout := qt.NewQVBoxLayout2()
	layout.SetContentsMargins(24, 22, 24, 22)
	layout.SetSpacing(14)
	container.SetLayout(layout.QLayout)
	title := qt.NewQLabel5("فاکتور "+invoice.Number+" با موفقیت صادر شد", container)
	setInvoiceObjectName(title, "pageTitle")
	layout.AddWidget(title.QWidget)
	hint := qt.NewQLabel5("پیش‌نمایش PDF را ببینید، یا فایل را برای اشتراک‌گذاری آماده کنید.", container)
	hint.SetWordWrap(true)
	layout.AddWidget(hint.QWidget)
	row := qt.NewQHBoxLayout2()
	preview := qt.NewQPushButton5("پیش‌نمایش فاکتور", container)
	share := qt.NewQPushButton5("اشتراک فاکتور", container)
	pdf := qt.NewQPushButton5("خروجی PDF", container)
	excel := qt.NewQPushButton5("خروجی Excel", container)
	back := qt.NewQPushButton5("بازگشت به فهرست فاکتورها", container)
	setInvoiceObjectName(preview, "secondaryAction")
	setInvoiceObjectName(share, "primaryAction")
	row.AddWidget(preview.QWidget)
	row.AddWidget(share.QWidget)
	row.AddWidget(pdf.QWidget)
	row.AddWidget(excel.QWidget)
	layout.AddLayout(row.QLayout)
	layout.AddWidget(back.QWidget)
	preview.OnClicked(func() {
		path, err := cachedInvoicePDF(ctx, invoice.ID, contacts, invoices, templates)
		if err != nil {
			qt.QMessageBox_Critical(window.QWidget, "ساخت پیش‌نمایش ناموفق بود", err.Error())
			return
		}
		showInvoicePDFPreview(window.QWidget, path, invoice.Number)
	})
	share.OnClicked(func() { showInvoiceShareOptions(window.QWidget, invoice, phone, ctx, contacts, invoices, templates) })
	pdf.OnClicked(func() {
		path := qt.QFileDialog_GetSaveFileName4(window.QWidget, "ذخیره فاکتور به PDF", invoice.Number+".pdf", "PDF (*.pdf)")
		if path == "" {
			return
		}
		if strings.ToLower(filepath.Ext(path)) != ".pdf" {
			path += ".pdf"
		}
		dir, name, err := findPDFFont()
		if err == nil {
			err = exportInvoiceFile(ctx, invoice.ID, path, "pdf", contacts, invoices, templates, dir, name)
		}
		if err != nil {
			qt.QMessageBox_Critical(window.QWidget, "خروجی PDF ناموفق بود", err.Error())
			return
		}
		qt.QMessageBox_Information(window.QWidget, "خروجی آماده شد", "فایل PDF ذخیره شد:\n"+path)
	})
	excel.OnClicked(func() {
		path := qt.QFileDialog_GetSaveFileName4(window.QWidget, "ذخیره فاکتور به Excel", invoice.Number+".xlsx", "Excel Workbook (*.xlsx)")
		if path == "" {
			return
		}
		if strings.ToLower(filepath.Ext(path)) != ".xlsx" {
			path += ".xlsx"
		}
		if err := exportInvoiceFile(ctx, invoice.ID, path, "xlsx", contacts, invoices, templates, ""); err != nil {
			qt.QMessageBox_Critical(window.QWidget, "خروجی Excel ناموفق بود", err.Error())
			return
		}
		qt.QMessageBox_Information(window.QWidget, "خروجی آماده شد", "فایل Excel ذخیره شد:\n"+path)
	})
	back.OnClicked(func() {
		window.Close()
		if onReturn != nil {
			onReturn()
		}
	})
	window.SetCentralWidget(container)
	window.Show()
	_ = parent
}

func cachedInvoicePDF(ctx context.Context, invoiceID int64, contacts *application.Contacts, invoices *application.Invoices, templates *application.Templates) (string, error) {
	if templates == nil {
		return "", fmt.Errorf("قالب خروجی برای ساخت PDF در دسترس نیست")
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, "invoice-maker", "shared")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("ساخت پوشهٔ PDF: %w", err)
	}
	doc, err := invoices.Get(ctx, invoiceID)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, doc.Number+".pdf")
	fontDir, fontName, err := findPDFFont()
	if err != nil {
		return "", err
	}
	if err := exportInvoiceFile(ctx, invoiceID, path, "pdf", contacts, invoices, templates, fontDir, fontName); err != nil {
		return "", err
	}
	return path, nil
}

func showInvoicePDFPreview(parent *qt.QWidget, path, number string) {
	window := qt.NewQMainWindow2()
	window.QWidget.SetWindowTitle("پیش‌نمایش فاکتور " + number)
	window.QWidget.SetMinimumSize2(760, 640)
	document := qtpdf.NewQPdfDocument()
	if result := document.Load(path); result != qtpdf.QPdfDocument__None {
		qt.QMessageBox_Critical(parent, "بازکردن PDF ناموفق بود", "فایل PDF تولید شد اما پیش‌نمایش داخلی در دسترس نیست.")
		return
	}
	viewer := qtpdf.NewQPdfView(window.QWidget)
	viewer.SetDocument(document)
	viewer.SetZoomMode(qtpdf.QPdfView__FitInView)
	window.SetCentralWidget(viewer.QWidget)
	window.Show()
}

func showInvoiceShareOptions(parent *qt.QWidget, invoice domain.InvoiceDocument, phone string, ctx context.Context, contacts *application.Contacts, invoices *application.Invoices, templates *application.Templates) {
	window := qt.NewQMainWindow2()
	window.QWidget.SetWindowTitle("اشتراک فاکتور " + invoice.Number)
	window.QWidget.SetMinimumSize2(520, 430)
	container := qt.NewQWidget(window.QWidget)
	layout := qt.NewQVBoxLayout2()
	layout.SetContentsMargins(22, 20, 22, 20)
	layout.SetSpacing(12)
	container.SetLayout(layout.QLayout)
	layout.AddWidget(sectionHeading(container, "انتخاب پیام‌رسان"))
	phoneLabel := "شمارهٔ مشتری در فاکتور ثبت نشده است."
	if strings.TrimSpace(phone) != "" {
		phoneLabel = "شمارهٔ مشتری: " + phone
	}
	phoneInfo := qt.NewQLabel5(phoneLabel, container)
	phoneInfo.SetWordWrap(true)
	layout.AddWidget(phoneInfo.QWidget)
	hint := qt.NewQLabel5("پس از انتخاب پیام‌رسان، فایل PDF را ذخیره می‌کنید و گفت‌وگو باز می‌شود. فایل را در پیام‌رسان پیوست و ارسال کنید.", container)
	hint.SetWordWrap(true)
	layout.AddWidget(hint.QWidget)
	status := qt.NewQLabel5("", container)
	status.SetWordWrap(true)
	layout.AddWidget(status.QWidget)
	platforms := []struct{ label, key string }{{"WhatsApp", "whatsapp"}, {"بله", "bale"}, {"تلگرام", "telegram"}, {"روبیکا", "rubika"}}
	for _, platform := range platforms {
		item := platform
		button := qt.NewQPushButton5(item.label, container)
		setInvoiceObjectName(button, "secondaryAction")
		button.OnClicked(func() {
			path := qt.QFileDialog_GetSaveFileName4(window.QWidget, "ذخیره PDF فاکتور برای اشتراک", invoice.Number+".pdf", "PDF (*.pdf)")
			if strings.TrimSpace(path) == "" {
				return
			}
			if strings.ToLower(filepath.Ext(path)) != ".pdf" {
				path += ".pdf"
			}
			fontDir, fontName, err := findPDFFont()
			if err == nil {
				err = exportInvoiceFile(ctx, invoice.ID, path, "pdf", contacts, invoices, templates, fontDir, fontName)
			}
			if err != nil {
				status.SetText("ساخت PDF ناموفق بود: " + err.Error())
				return
			}
			if strings.TrimSpace(phone) != "" {
				qt.QGuiApplication_Clipboard().SetText(phone)
			}
			if strings.TrimSpace(phone) == "" {
				status.SetText("فایل ذخیره شد؛ شمارهٔ مشتری در فاکتور موجود نیست. پیام‌رسان باز می‌شود تا مخاطب را دستی انتخاب کنید.")
			}
			opened := openShareTarget(item.key, phone, invoice.Number)
			message := "فایل ذخیره شد: " + path + "\nگفت‌وگو باز شد؛ فایل PDF را در پیام‌رسان پیوست کنید."
			if !opened {
				message = "فایل ذخیره شد: " + path + "\nبازکردن پیام‌رسان ناموفق بود؛ برنامه یا وب‌اپ آن را باز کنید و PDF را پیوست کنید."
			}
			if strings.TrimSpace(phone) != "" {
				message += " شمارهٔ مشتری هم برای جای‌گذاری در کلیپ‌بورد قرار گرفت."
			} else {
				message += " شمارهٔ مشتری در snapshot فاکتور نبود؛ مخاطب را دستی انتخاب کنید."
			}
			status.SetText(message)
		})
		layout.AddWidget(button.QWidget)
	}
	close := qt.NewQPushButton5("بستن", container)
	close.OnClicked(func() { window.Close() })
	layout.AddWidget(close.QWidget)
	window.SetCentralWidget(container)
	window.Show()
	_ = parent
}

func openShareTarget(platform, phone, invoiceNumber string) bool {
	var digits strings.Builder
	for _, r := range phone {
		switch {
		case r >= '0' && r <= '9':
			digits.WriteRune(r)
		case r >= '۰' && r <= '۹':
			digits.WriteRune('0' + (r - '۰'))
		case r >= '٠' && r <= '٩':
			digits.WriteRune('0' + (r - '٠'))
		}
	}
	cleanPhone := digits.String()
	cleanPhone = strings.TrimPrefix(cleanPhone, "+")
	if strings.HasPrefix(cleanPhone, "0") {
		cleanPhone = "98" + strings.TrimPrefix(cleanPhone, "0")
	}
	if cleanPhone != "" && !strings.HasPrefix(cleanPhone, "98") && len(cleanPhone) >= 10 {
		cleanPhone = "98" + cleanPhone
	}
	message := url.QueryEscape("سلام، فاکتور " + invoiceNumber + " را برای شما ارسال می‌کنم.")
	var target string
	switch platform {
	case "whatsapp":
		if cleanPhone != "" {
			target = "whatsapp://send?phone=" + cleanPhone + "&text=" + message
			if qt.QDesktopServices_OpenUrl(qt.NewQUrl3(target)) {
				return true
			}
			target = "https://wa.me/" + cleanPhone + "?text=" + message
		} else {
			target = "https://web.whatsapp.com/"
		}
	case "telegram":
		if cleanPhone != "" {
			target = "tg://resolve?phone=" + cleanPhone + "&text=" + message
			if qt.QDesktopServices_OpenUrl(qt.NewQUrl3(target)) {
				return true
			}
			target = "https://t.me/+" + cleanPhone + "?text=" + message
		} else {
			target = "https://web.telegram.org/"
		}
	case "bale":
		target = "https://web.bale.ai/"
	case "rubika":
		target = "https://web.rubika.ir/"
	}
	return target != "" && qt.QDesktopServices_OpenUrl(qt.NewQUrl3(target))
}
