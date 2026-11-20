package main

import (
	"context"
	"time"

	"github.com/hesora-ir/hesora-free/internal/domain"
	qt "github.com/mappu/miqt/qt6"
)

// showOnlineInvoiceShare publishes a snapshot and exposes its public link.
func showOnlineInvoiceShare(parent *qt.QWidget, invoice domain.InvoiceDocument) {
	window := qt.NewQMainWindow2()
	window.QWidget.SetWindowTitle("اشتراک آنلاین فاکتور " + invoice.Number)
	window.QWidget.SetMinimumSize2(520, 260)
	container := qt.NewQWidget(window.QWidget)
	layout := qt.NewQVBoxLayout2()
	layout.SetContentsMargins(22, 20, 22, 20)
	layout.SetSpacing(12)
	container.SetLayout(layout.QLayout)
	layout.AddWidget(sectionHeading(container, "مشاهده و اشتراک آنلاین"))
	status := qt.NewQLabel5("در حال ساخت لینک فاکتور…", container)
	status.SetWordWrap(true)
	layout.AddWidget(status.QWidget)
	linkInput := qt.NewQLineEdit(container)
	linkInput.SetReadOnly(true)
	linkInput.SetLayoutDirection(qt.LeftToRight)
	layout.AddWidget(linkInput.QWidget)
	copyButton := qt.NewQPushButton5("کپی لینک", container)
	openButton := qt.NewQPushButton5("مشاهده در مرورگر", container)
	copyButton.SetEnabled(false)
	openButton.SetEnabled(false)
	layout.AddWidget(copyButton.QWidget)
	layout.AddWidget(openButton.QWidget)
	copyButton.OnClicked(func() {
		qt.QGuiApplication_Clipboard().SetText(linkInput.Text())
		status.SetText("لینک کپی شد و می‌توانید آن را برای مشتری بفرستید.")
	})
	openButton.OnClicked(func() {
		if !qt.QDesktopServices_OpenUrl(qt.NewQUrl3(linkInput.Text())) {
			status.SetText("بازکردن مرورگر ناموفق بود؛ لینک را کپی و در مرورگر وارد کنید.")
		}
	})
	window.SetCentralWidget(container)
	window.Show()

	client := activeCloudClient
	if client == nil {
		status.SetText("اتصال به حساب برقرار نیست. لایسنس را فعال و اتصال اینترنت را بررسی کنید.")
		return
	}
	type result struct {
		link string
		err  error
	}
	results := make(chan result, 1)
	timer := qt.NewQTimer2(window.QObject)
	timer.OnTimeout(func() {
		select {
		case r := <-results:
			timer.Stop()
			if r.err != nil {
				status.SetText("ساخت لینک ناموفق بود: " + r.err.Error())
				return
			}
			linkInput.SetText(r.link)
			qt.QGuiApplication_Clipboard().SetText(r.link)
			copyButton.SetEnabled(true)
			openButton.SetEnabled(true)
			status.SetText("لینک عمومی ساخته و کپی شد. ساخت لینک جدید، لینک قبلی این فاکتور را غیرفعال می‌کند.")
		default:
		}
	})
	timer.Start(100)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		link, err := client.ShareInvoice(ctx, invoiceSharePayload(invoice))
		results <- result{link, err}
	}()
	_ = parent
}
