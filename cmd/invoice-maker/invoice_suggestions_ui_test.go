package main

import (
	"os"
	"runtime"
	"testing"

	"github.com/hesora-ir/hesora-free/internal/domain"
	qt "github.com/mappu/miqt/qt6"
)

func TestInvoiceSuggestionSelectionQt(t *testing.T) {
	if os.Getenv("INVOICE_QT_TEST") == "" {
		t.Skip("opt-in Qt interaction test")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	qt.NewQApplication([]string{"invoice-suggestion-test", "-platform", "offscreen"})
	row := qt.NewQWidget2()
	name, unit, price := qt.NewQLineEdit(row), qt.NewQLineEdit(row), qt.NewQLineEdit(row)
	item := domain.InvoiceItem{Description: "پچ پنل طرح لگراند 24 پورت SFTP", Unit: "عدد", UnitPrice: 17300000}
	completer := attachInvoiceItemSuggestions(row, name, unit, price, "تومان", func(string) ([]domain.InvoiceItem, error) {
		return []domain.InvoiceItem{item}, nil
	}, func() {})
	name.SetText("پچ")
	name.TextEdited("پچ")
	label := item.Description + " — " + formatInvoiceMoney(int64(item.UnitPrice)) + " تومان"
	// Simulate the line edit inserting the popup text before activation.
	name.SetText(label)
	name.TextEdited(label)
	completer.Activated(label)
	qt.QCoreApplication_ProcessEvents()
	if name.Text() != item.Description || unit.Text() != item.Unit || price.Text() != "17300000" {
		t.Fatalf("selected name=%q unit=%q price=%q", name.Text(), unit.Text(), price.Text())
	}
}
