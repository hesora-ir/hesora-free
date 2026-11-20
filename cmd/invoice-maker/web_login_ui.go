package main

import (
	"context"
	"time"

	qt "github.com/mappu/miqt/qt6"
)

// addWebLoginButton exchanges the current device session for a fresh one-use URL.
func addWebLoginButton(parent *qt.QWidget, layout *qt.QVBoxLayout) {
	button := qt.NewQPushButton5("ورود بدون رمز به نسخهٔ وب", parent)
	setInvoiceObjectName(button, "navItem")
	layout.AddWidget(button.QWidget)
	type result struct {
		url string
		err error
	}
	results := make(chan result, 1)
	timer := qt.NewQTimer2(parent.QObject)
	timer.OnTimeout(func() {
		select {
		case r := <-results:
			timer.Stop()
			button.SetEnabled(true)
			if r.err != nil {
				qt.QMessageBox_Warning(parent, "ورود به نسخهٔ وب", "دریافت پیوند ناموفق بود: "+r.err.Error())
				return
			}
			if !qt.QDesktopServices_OpenUrl(qt.NewQUrl3(r.url)) {
				qt.QMessageBox_Warning(parent, "ورود به نسخهٔ وب", "باز کردن مرورگر ممکن نشد. دوباره تلاش کنید.")
			}
		default:
		}
	})
	button.OnClicked(func() {
		client := activeCloudClient
		if client == nil {
			qt.QMessageBox_Information(parent, "ورود به نسخهٔ وب", "اتصال حساب برقرار نیست. ابتدا لایسنس خریداری‌شده را فعال کنید و از اتصال اینترنت مطمئن شوید.")
			return
		}
		button.SetEnabled(false)
		timer.Start(100)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			url, err := client.MagicLink(ctx)
			results <- result{url, err}
		}()
	})
}
