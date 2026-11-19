package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hesora-ir/hesora-free/internal/adapters/sqlite"
	"github.com/hesora-ir/hesora-free/internal/domain"
	qt "github.com/mappu/miqt/qt6"
)

func buildExpensesPage(parent *qt.QWidget, store *sqlite.Store, ctx context.Context) page {
	p := newPage(parent, "هزینه‌ها", "ثبت و پیگیری هزینه‌های فروشگاه؛ هزینه‌ها همراه با همگام‌سازی ابری بین وب و دسکتاپ جابه‌جا می‌شوند.")
	form, layout := newSurface(p.widget, "ثبت هزینه", "مبلغ را به واحد پول انتخاب‌شده وارد کنید.")
	p.layout.AddWidget(form)
	amount := qt.NewQLineEdit(form)
	amount.SetPlaceholderText("مبلغ")
	category := qt.NewQLineEdit(form)
	category.SetPlaceholderText("دسته‌بندی هزینه")
	date := qt.NewQLineEdit(form)
	date.SetText(time.Now().Format("2006-01-02"))
	date.SetPlaceholderText("تاریخ YYYY-MM-DD")
	currency := qt.NewQComboBox(form)
	currency.AddItem("تومان (IRT)")
	currency.AddItem("ریال (IRR)")
	for _, w := range []*qt.QWidget{amount.QWidget, category.QWidget, date.QWidget, currency.QWidget} {
		layout.AddWidget(w)
	}
	note := qt.NewQLineEdit(form)
	note.SetPlaceholderText("یادداشت (اختیاری)")
	layout.AddWidget(note.QWidget)
	status := qt.NewQLabel5("", form)
	status.SetWordWrap(true)
	layout.AddWidget(status.QWidget)
	list, listLayout := newSurface(p.widget, "سوابق هزینه", "هزینه‌های ذخیره‌شده در این دستگاه و فضای ابری")
	p.layout.AddWidget(list)
	var editingID int64
	var refresh func()
	refresh = func() {
		for i := listLayout.Count() - 1; i >= 0; i-- {
			item := listLayout.ItemAt(i)
			if item != nil && item.Widget() != nil {
				listLayout.RemoveWidget(item.Widget())
				item.Widget().DeleteLater()
			}
		}
		rows, err := store.ListExpenses(ctx)
		if err != nil {
			status.SetText("خواندن هزینه‌ها ناموفق بود: " + err.Error())
			return
		}
		if len(rows) == 0 {
			listLayout.AddWidget(qt.NewQLabel5("هنوز هزینه‌ای ثبت نشده است.", list).QWidget)
		}
		for _, e := range rows {
			label := qt.NewQLabel5(fmt.Sprintf("%s  ·  %s  ·  %d %s", e.Date.Format("2006-01-02"), e.Category, e.Amount, e.Currency), list)
			label.SetWordWrap(true)
			edit := qt.NewQPushButton5("ویرایش", list)
			id := e.ID
			edit.OnClicked(func() {
				editingID = id
				amount.SetText(fmt.Sprint(e.Amount))
				category.SetText(e.Category)
				date.SetText(e.Date.Format("2006-01-02"))
				note.SetText(e.Note)
				if e.Currency == domain.CurrencyRial {
					currency.SetCurrentIndex(1)
				} else {
					currency.SetCurrentIndex(0)
				}
				status.SetText("هزینه برای ویرایش انتخاب شد.")
			})
			remove := qt.NewQPushButton5("حذف", list)
			remove.OnClicked(func() {
				if qt.QMessageBox_Question2(list, "حذف هزینه", "این هزینه از سوابق این حساب حذف شود؟", qt.QMessageBox__Yes, qt.QMessageBox__No) != int(qt.QMessageBox__Yes) {
					return
				}
				if err := store.DeleteExpense(ctx, id); err != nil {
					status.SetText("حذف هزینه ناموفق بود: " + err.Error())
					return
				}
				editingID = 0
				refresh()
			})
			listLayout.AddWidget(label.QWidget)
			listLayout.AddWidget(edit.QWidget)
			listLayout.AddWidget(remove.QWidget)
		}
	}
	save := qt.NewQPushButton5("ثبت هزینه", form)
	setInvoiceObjectName(save, "primaryAction")
	layout.AddWidget(save.QWidget)
	save.OnClicked(func() {
		value, err := strconv.ParseInt(strings.ReplaceAll(strings.TrimSpace(amount.Text()), ",", ""), 10, 64)
		if err != nil {
			status.SetText("مبلغ را به‌صورت عدد صحیح وارد کنید.")
			return
		}
		day, err := time.Parse("2006-01-02", strings.TrimSpace(date.Text()))
		if err != nil {
			status.SetText("تاریخ باید به شکل YYYY-MM-DD باشد.")
			return
		}
		cur := domain.CurrencyToman
		if currency.CurrentIndex() == 1 {
			cur = domain.CurrencyRial
		}
		_, err = store.SaveExpense(ctx, domain.Expense{ID: editingID, Amount: domain.Money(value), Currency: cur, Date: day, Category: strings.TrimSpace(category.Text()), Note: strings.TrimSpace(note.Text())})
		if err != nil {
			status.SetText("ذخیره هزینه ناموفق بود: " + err.Error())
			return
		}
		amount.Clear()
		category.Clear()
		note.Clear()
		editingID = 0
		status.SetText("هزینه ذخیره شد.")
		refresh()
	})
	refresh()
	p.layout.AddStretch()
	return p
}
