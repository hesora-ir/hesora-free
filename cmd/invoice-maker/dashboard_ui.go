package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hesora-ir/hesora-free/internal/application"
	"github.com/hesora-ir/hesora-free/internal/domain"
	qt "github.com/mappu/miqt/qt6"
)

func buildDashboardPage(parent *qt.QWidget, service *application.Dashboard, ctx context.Context, navigate func(int)) (page, func()) {
	p := newPage(parent, "نمای کلی", "وضعیت فاکتورها و دریافت‌ها در یک نگاه • از ابتدای فعالیت")
	header := qt.NewQHBoxLayout2()
	status := qt.NewQLabel5("", p.widget)
	status.SetWordWrap(true)
	status.SetMinimumWidth(450)
	setInvoiceObjectName(status, "muted")
	refreshButton := qt.NewQPushButton5("به‌روزرسانی آمار", p.widget)
	setInvoiceObjectName(refreshButton, "secondaryAction")
	header.AddWidget(status.QWidget)
	header.AddStretch()
	header.AddWidget(refreshButton.QWidget)
	p.layout.AddLayout(header.QLayout)
	counts := qt.NewQHBoxLayout2()
	p.layout.AddLayout(counts.QLayout)
	unsettled := dashboardKPI(counts, p.widget, "فاکتورهای تسویه‌نشده", "فاکتورهای صادرشده با مانده مثبت")
	customers := dashboardKPI(counts, p.widget, "تعداد مشتریان", "همه مشتریان ثبت‌شده")
	settled := dashboardKPI(counts, p.widget, "فاکتورهای تسویه‌شده", "فاکتورهای صادرشده با مانده صفر")
	amounts := qt.NewQHBoxLayout2()
	p.layout.AddLayout(amounts.QLayout)
	received := dashboardKPI(amounts, p.widget, "مبلغ دریافتی", "جمع پرداخت‌های ثبت‌شده")
	outstanding := dashboardKPI(amounts, p.widget, "مبلغ مانده", "فقط فاکتورهای نهایی صادرشده")
	p.layout.AddWidget(sectionHeading(p.widget, "دسترسی سریع"))
	actions := qt.NewQHBoxLayout2()
	for i, title := range []string{"صدور فاکتور", "مشاهده فاکتورها", "ثبت مشتری"} {
		index := i + 1
		button := qt.NewQPushButton5(title, p.widget)
		if i > 0 {
			setInvoiceObjectName(button, "secondaryAction")
		}
		button.OnClicked(func() { navigate(index) })
		actions.AddWidget(button.QWidget)
	}
	p.layout.AddLayout(actions.QLayout)
	activityPanel, activityLayout := newSurface(p.widget, "آخرین فعالیت‌ها", "۸ رویداد آخر ثبت، ویرایش، صدور و پرداخت اسناد")
	p.layout.AddWidget(activityPanel)
	activity := qt.NewQLabel5("", activityPanel)
	activity.SetWordWrap(true)
	activityLayout.AddWidget(activity.QWidget)
	p.layout.AddStretch()
	refresh := func() {
		data, err := service.Load(ctx)
		if err != nil {
			for _, label := range []*qt.QLabel{unsettled, customers, settled, received, outstanding} {
				label.SetText("—")
			}
			activity.SetText("فعالیت‌ها بارگذاری نشدند.")
			status.SetText("خواندن آمار انجام نشد: " + err.Error())
			return
		}
		unsettled.SetText(formatCustomerMoney(data.Unsettled))
		customers.SetText(formatCustomerMoney(data.Customers))
		settled.SetText(formatCustomerMoney(data.Settled))
		received.SetText(dashboardMoney(data.Toman.Received, data.Rial.Received))
		outstanding.SetText(dashboardMoney(data.Toman.Outstanding, data.Rial.Outstanding))
		lines := make([]string, 0, len(data.Activities))
		actions := map[string]string{"created": "ثبت سند", "issued": "صدور فاکتور", "draft_updated": "ویرایش پیش‌نویس", "proforma_converted": "تبدیل پیش‌فاکتور", "payment_allocated": "ثبت پرداخت"}
		for _, event := range data.Activities {
			local := event.OccurredAt.Local()
			date, _ := domain.GregorianToJalali(local.Year(), int(local.Month()), local.Day())
			lines = append(lines, fmt.Sprintf("%s  •  %s  •  %04d/%02d/%02d  %s", actions[event.Action], event.Number, date.Year, date.Month, date.Day, local.Format("15:04")))
		}
		if len(lines) == 0 {
			lines = append(lines, "هنوز فعالیتی ثبت نشده است. با ثبت مشتری و صدور اولین فاکتور شروع کنید.")
		}
		activity.SetText(strings.Join(lines, "\n\n"))
		status.SetText("آخرین به‌روزرسانی: " + time.Now().Format("15:04") + " • ریال و تومان به‌صورت جداگانه")
	}
	refreshButton.OnClicked(refresh)
	scroll := qt.NewQScrollArea(parent)
	scroll.SetWidgetResizable(true)
	scroll.SetWidget(p.widget)
	return page{widget: scroll.QWidget, layout: p.layout}, refresh
}

func dashboardMoney(toman, rial domain.Money) string {
	return formatInvoiceMoney(int64(toman)) + " تومان\n" + formatInvoiceMoney(int64(rial)) + " ریال"
}

func dashboardKPI(row *qt.QHBoxLayout, parent *qt.QWidget, title, note string) *qt.QLabel {
	card, layout := newSurface(parent, title, "")
	value := qt.NewQLabel5("—", card)
	setInvoiceObjectName(value, "metricValue")
	value.SetWordWrap(true)
	value.SetAlignment(qt.AlignRight | qt.AlignAbsolute | qt.AlignVCenter)
	layout.AddWidget(value.QWidget)
	caption := qt.NewQLabel5(note, card)
	setInvoiceObjectName(caption, "muted")
	caption.SetWordWrap(true)
	layout.AddWidget(caption.QWidget)
	row.AddWidget(card)
	return value
}

func newSurface(parent *qt.QWidget, title, subtitle string) (*qt.QWidget, *qt.QVBoxLayout) {
	w := qt.NewQWidget(parent)
	setInvoiceObjectName(w, "surface")
	l := qt.NewQVBoxLayout2()
	l.SetContentsMargins(18, 16, 18, 16)
	l.SetSpacing(12)
	w.SetLayout(l.QLayout)
	h := qt.NewQLabel5(title, w)
	setInvoiceObjectName(h, "sectionTitle")
	l.AddWidget(h.QWidget)
	if subtitle != "" {
		s := qt.NewQLabel5(subtitle, w)
		s.SetWordWrap(true)
		setInvoiceObjectName(s, "muted")
		l.AddWidget(s.QWidget)
	}
	return w, l
}

func labeledInvoiceField(row *qt.QHBoxLayout, parent *qt.QWidget, label string, input *qt.QWidget) {
	w := qt.NewQWidget(parent)
	setInvoiceObjectName(w, "fieldGroup")
	l := qt.NewQVBoxLayout2()
	l.SetContentsMargins(0, 0, 0, 0)
	l.SetSpacing(6)
	w.SetLayout(l.QLayout)
	caption := qt.NewQLabel5(label, w)
	input.SetAccessibleName(label)
	l.AddWidget(caption.QWidget)
	l.AddWidget(input)
	row.AddWidget(w)
}
