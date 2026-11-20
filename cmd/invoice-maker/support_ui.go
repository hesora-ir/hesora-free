package main

import (
	"context"
	qt "github.com/mappu/miqt/qt6"
	"strings"
	"time"
)

func buildSupportPage(parent *qt.QWidget) page {
	p := newPage(parent, "درخواست پشتیبانی", "درخواست خود را برای تیم پشتیبانی ارسال کنید.")
	subject := qt.NewQLineEdit(p.widget)
	subject.SetPlaceholderText("عنوان درخواست")
	body := qt.NewQTextEdit(p.widget)
	body.SetPlaceholderText("شرح درخواست پشتیبانی")
	body.SetMinimumHeight(160)
	submit := qt.NewQPushButton5("ارسال درخواست", p.widget)
	status := qt.NewQLabel5("برای ارسال درخواست، لایسنس خود را فعال کنید.", p.widget)
	status.SetWordWrap(true)
	p.layout.AddWidget(subject.QWidget)
	p.layout.AddWidget(body.QWidget)
	p.layout.AddWidget(submit.QWidget)
	p.layout.AddWidget(status.QWidget)
	p.layout.AddStretch()
	type result struct {
		id  string
		err error
	}
	results := make(chan result, 1)
	timer := qt.NewQTimer2(p.widget.QObject)
	timer.OnTimeout(func() {
		select {
		case r := <-results:
			timer.Stop()
			submit.SetEnabled(true)
			if r.err != nil {
				status.SetText("ارسال درخواست ناموفق بود: " + r.err.Error())
				return
			}
			subject.Clear()
			body.Clear()
			status.SetText("درخواست پشتیبانی ثبت شد: " + r.id)
		default:
		}
	})
	submit.OnClicked(func() {
		title, message := strings.TrimSpace(subject.Text()), strings.TrimSpace(body.ToPlainText())
		if title == "" || message == "" {
			status.SetText("عنوان و شرح درخواست را وارد کنید.")
			return
		}
		client := activeCloudClient
		if client == nil {
			status.SetText("اتصال پشتیبانی برقرار نیست؛ لایسنس را فعال کنید و دوباره تلاش کنید.")
			return
		}
		submit.SetEnabled(false)
		status.SetText("در حال ارسال درخواست…")
		timer.Start(100)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			id, err := client.CreateSupportTicket(ctx, title, message)
			results <- result{id, err}
		}()
	})
	return p
}
