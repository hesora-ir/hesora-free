package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hesora-ir/hesora-free/internal/adapters/sqlite"
	"github.com/hesora-ir/hesora-free/internal/application"
	"github.com/hesora-ir/hesora-free/internal/domain"
	"github.com/hesora-ir/hesora-free/internal/licensing"
	qt "github.com/mappu/miqt/qt6"
)

// Opt-in rendering check: all fixtures and images stay in a disposable database.
func TestUIReview(t *testing.T) {
	if os.Getenv("INVOICE_UI_REVIEW") == "" {
		t.Skip("opt-in Qt rendering review")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	qt.NewQApplication([]string{"ui-review", "-platform", "offscreen"})
	qt.QGuiApplication_SetLayoutDirection(qt.RightToLeft)
	id := qt.QFontDatabase_AddApplicationFont("../../assets/font/iransans/IRANSansX-Regular.ttf")
	family := "Tahoma"
	if fonts := qt.QFontDatabase_ApplicationFontFamilies(id); len(fonts) > 0 {
		family = fonts[0]
	}
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "review.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	contacts, invoices := application.NewContacts(store), application.NewInvoices(store)
	doc := domain.InvoiceDocument{Kind: "proforma", Status: domain.InvoiceDraft, Currency: domain.CurrencyToman,
		Items:           []domain.InvoiceItem{{Description: "طراحی و پیاده‌سازی وب‌سایت", Quantity: 2, Unit: "ساعت", UnitPrice: 250000}},
		InvoiceDiscount: domain.Discount{Mode: domain.DiscountFixed, Value: 50000}, TaxRateBPS: 900, Note: "تحویل سفارش پس از تأیید پیش‌فاکتور"}
	output := "../../build/ui-review"
	if err := os.MkdirAll(output, 0700); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{false, true} {
		root := qt.NewQWidget2()
		style, suffix := lightAppStyle(family, 10), "light"
		if dark {
			style, suffix = darkAppStyle(family, 10), "dark"
		}
		root.SetStyleSheet(style)
		layout := qt.NewQVBoxLayout2()
		root.SetLayout(layout.QLayout)
		dashboard, refresh := buildDashboardPage(root, application.NewDashboard(store), ctx, func(int) {})
		refresh()
		layout.AddWidget(dashboard.widget)
		root.Resize(1120, 900)
		root.Show()
		qt.QCoreApplication_ProcessEvents()
		if !root.Grab().Save(filepath.Join(output, "dashboard-"+suffix+".png")) {
			t.Fatal("dashboard capture failed")
		}
		layout.RemoveWidget(dashboard.widget)
		dashboard.widget.Hide()
		editor := buildInvoicePage(root, contacts, invoices, ctx, &doc)
		layout.AddWidget(editor.widget)
		root.Resize(1120, 1800)
		qt.QCoreApplication_ProcessEvents()
		if !root.Grab().Save(filepath.Join(output, "invoice-"+suffix+".png")) {
			t.Fatal("invoice capture failed")
		}
		layout.RemoveWidget(editor.widget)
		editor.widget.Hide()
		issuer := buildIssuerFormPage(root, contacts, ctx)
		layout.AddWidget(issuer.widget)
		qt.QCoreApplication_ProcessEvents()
		if !root.Grab().Save(filepath.Join(output, "issuer-"+suffix+".png")) {
			t.Fatal("issuer capture failed")
		}
		layout.RemoveWidget(issuer.widget)
		issuer.widget.Hide()
		cloudPage := qt.NewQWidget(root)
		cloudLayout := qt.NewQVBoxLayout2()
		cloudPage.SetLayout(cloudLayout.QLayout)
		addCloudAndBackupSettings(cloudPage, cloudLayout, store, licensing.FileStore{Directory: filepath.Dir(store.Path())}, "0123456789abcdef0123456789abcdef", func() {})
		layout.AddWidget(cloudPage)
		root.Resize(1120, 1000)
		qt.QCoreApplication_ProcessEvents()
		if !root.Grab().Save(filepath.Join(output, "cloud-"+suffix+".png")) {
			t.Fatal("cloud settings capture failed")
		}
		root.Hide()
	}
}
