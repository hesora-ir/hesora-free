package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"os"
	osExec "os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hesora-ir/hesora-free/internal/adapters/sqlite"
	"github.com/hesora-ir/hesora-free/internal/application"
	"github.com/hesora-ir/hesora-free/internal/cloud"
	"github.com/hesora-ir/hesora-free/internal/domain"
	"github.com/hesora-ir/hesora-free/internal/licensing"
	qt "github.com/mappu/miqt/qt6"
)

const appStyle = `
QMainWindow, QWidget { background:#f3f5f8; color:#1d2a35; font-family:"IRANSansX","Tahoma"; font-size:10pt; }
QTabWidget::pane { border:0; background:#f3f5f8; }
QTabBar { qproperty-drawBase:0; }
QLineEdit,QComboBox,QDateEdit,QSpinBox { background:white; border:1px solid #e1e6ed; border-radius:9px; padding:10px 12px; min-height:20px; selection-background-color:#7058ed; }
QLineEdit:focus,QComboBox:focus { border:1px solid #7058ed; }
QPushButton { background:#7058ed; color:white; border:0; border-radius:9px; padding:10px 16px; min-height:20px; font-weight:600; }
QPushButton:hover { background:#604bd8; }
QPushButton:pressed { background:#5140c5; }
QLabel#pageTitle { color:#1d2a35; font-size:19pt; font-weight:700; }
QLabel#muted { color:#788493; }
QWidget#appSidebar { background:#192631; border:0; }
QLabel#brandMark { background:#7058ed; color:white; border-radius:12px; font-size:19pt; font-weight:700; padding:9px; }
QLabel#brandTitle { color:white; font-size:14pt; font-weight:700; }
QLabel#sidebarCaption { color:#8997a4; font-size:9pt; padding:8px 10px; }
QPushButton#navItem { background:transparent; color:#b9c4cf; border:0; border-radius:9px; text-align:right; padding:12px 14px; min-height:24px; }
QPushButton#navItem:hover { background:#253440; color:white; }
QPushButton#navItem:checked { background:#2c3946; color:white; font-weight:700; }
QPushButton#navItem:checked:hover { background:#354553; }
QWidget#appTopbar { background:white; border:1px solid #e5e9ef; border-radius:12px; }
QLineEdit#quickSearch { background:#f5f6f9; border:0; }
QWidget#appContent { background:#f3f5f8; }
QWidget#surface { background:white; border:1px solid #e5e9ef; border-radius:15px; }
`

// Set at release build time to the public key whose matching private key is held by Hesora.
var hesoraLicensePublicKeyBase64 string

func main() {
	ctx := context.Background()
	store, err := sqlite.OpenDefault(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "خطا در آماده‌سازی پایگاه داده:", err)
		os.Exit(1)
	}
	defer store.Close()

	contacts := application.NewContacts(store)
	invoices := application.NewInvoices(store)
	templates := application.NewTemplates(store)
	licenseFiles := licensing.FileStore{Directory: filepath.Dir(store.Path())}
	installationID, installationErr := licenseFiles.InstallationID()
	licensePublicKey, _ := base64.RawURLEncoding.DecodeString(hesoraLicensePublicKeyBase64)
	if len(licensePublicKey) != ed25519.PublicKeySize {
		licensePublicKey, _ = base64.StdEncoding.DecodeString(hesoraLicensePublicKeyBase64)
	}
	if len(licensePublicKey) != ed25519.PublicKeySize {
		if savedKey, err := licenseFiles.ReadPublicKey(); err == nil {
			licensePublicKey = savedKey
		}
	}
	verifier := licensing.Verifier{PublicKey: ed25519.PublicKey(licensePublicKey), InstallationID: installationID, FreeFeatures: licensing.DefaultFreeFeatures()}
	licenseToken, _ := licenseFiles.ReadToken()
	licenseResult := verifier.Verify(licenseToken)
	store.SetProEnabled(licenseResult.HasFeature(licensing.ProFullFeature))
	templates.SetProEnabled(store.ProEnabled())
	qt.NewQApplication(os.Args)
	qt.QGuiApplication_SetLayoutDirection(qt.RightToLeft)

	window := qt.NewQMainWindow2()
	window.QWidget.SetWindowTitle("Hesora — هسورا")
	window.QWidget.SetMinimumSize2(1120, 760)
	icon := qt.NewQIcon4(resolveApplicationIconPath())
	window.QWidget.SetWindowIcon(icon)
	qt.QGuiApplication_SetWindowIcon(icon)
	window.QWidget.SetStyleSheet(appStyle)

	tabs := qt.NewQTabWidget(window.QWidget)

	dashboard, refreshDashboard := buildDashboardPage(tabs.QWidget, application.NewDashboard(store), ctx, func(index int) { tabs.SetCurrentIndex(index) })

	// Invoice issuance and list pages use the application service and persisted SQLite data.
	invoicePage := buildInvoicePage(tabs.QWidget, contacts, invoices, ctx, nil, invoiceEditorOptions{templates: templates, onIssued: func() { tabs.SetCurrentIndex(2) }})
	invoicesPage := buildInvoicesPage(tabs.QWidget, contacts, invoices, templates, ctx)

	// Customer management: profile editing, summary cards, and invoice history.
	customerPage := buildCustomerPage(tabs.QWidget, contacts, invoices, templates, ctx)

	// Base information: issuer profiles.
	issuerPage := buildIssuerPage(tabs.QWidget, contacts, ctx)

	// Settings: persisted preferences, licensing, and the free/pro comparison.
	settingsPage := newPage(tabs.QWidget, "تنظیمات", "ظاهر برنامه، پیش‌فرض‌ها و وضعیت مجوز")
	settingsScroll := qt.NewQScrollArea(tabs.QWidget)
	settingsScroll.SetWidgetResizable(true)
	settingsScroll.SetWidget(settingsPage.widget)

	settingsPage.layout.AddWidget(sectionHeading(settingsPage.widget, "ظاهر برنامه"))
	fontSetting := qt.NewQComboBox(settingsPage.widget)
	fontSetting.AddItem("IRANSansX")
	fontSetting.AddItem("Tahoma (سیستم)")
	proFontChoices := []string{"IRANSharp", "IRAN Family Typeface", "IRAN Kharazmi", "IRANMarker", "Segoe UI (سیستم)"}
	if store.ProEnabled() {
		for _, choice := range proFontChoices {
			fontSetting.AddItem(choice)
		}
	}
	fontSizeSetting := qt.NewQSpinBox(settingsPage.widget)
	fontSizeSetting.SetRange(8, 32)
	fontSizeSetting.SetSuffix(" pt")
	fontRow := qt.NewQHBoxLayout2()
	settingsPage.layout.AddLayout(fontRow.QLayout)
	fontLabel := qt.NewQLabel5("فونت رابط", settingsPage.widget)
	fontRow.AddWidget(fontLabel.QWidget)
	fontRow.AddWidget(fontSetting.QWidget)
	fontSizeLabel := qt.NewQLabel5("اندازه", settingsPage.widget)
	fontRow.AddWidget(fontSizeLabel.QWidget)
	fontRow.AddWidget(fontSizeSetting.QWidget)

	settingsPage.layout.AddWidget(sectionHeading(settingsPage.widget, "قالب رنگ"))
	themeSetting := qt.NewQComboBox(settingsPage.widget)
	themeSetting.AddItem("روشن")
	themeSetting.AddItem("تیره")
	themeSetting.AddItem("سیستمی (فعلاً روشن)")
	settingsPage.layout.AddWidget(themeSetting.QWidget)

	settingsPage.layout.AddWidget(sectionHeading(settingsPage.widget, "پیش‌فرض‌های فاکتور"))
	currencySetting := qt.NewQComboBox(settingsPage.widget)
	currencySetting.AddItem("تومان (IRT)")
	currencySetting.AddItem("ریال (IRR)")
	settingsPage.layout.AddWidget(currencySetting.QWidget)

	settingsPage.layout.AddWidget(sectionHeading(settingsPage.widget, "مجوز نرم‌افزار"))
	licenseState := qt.NewQLabel5("Hesora رایگان", settingsPage.widget)
	if store.ProEnabled() {
		licenseState.SetText("Hesora Pro فعال")
	}
	licenseState.SetStyleSheet("color: #718198; font-weight: 700; padding: 10px; background: #edf2f8; border-radius: 8px;")
	licenseDescriptionText := "نسخهٔ رایگان شامل یک فروشگاه، حداکثر ۵۰ مشتری و ۲۰۰ فاکتور، قالب استاندارد و فونت‌های محدود است. اطلاعات بیشتر: hesora.ir"
	if store.ProEnabled() {
		licenseDescriptionText = "لایسنس پرو فعال است. برای بکاپ ابری و مشاهده وضعیت حساب، در بخش اتصال به فضای ابری وارد شوید."
	}
	licenseDescription := qt.NewQLabel5(licenseDescriptionText, settingsPage.widget)
	licenseDescription.SetWordWrap(true)
	settingsPage.layout.AddWidget(licenseState.QWidget)
	settingsPage.layout.AddWidget(licenseDescription.QWidget)
	licenseBuildStatus := qt.NewQLabel5("", settingsPage.widget)
	licenseBuildStatus.SetWordWrap(true)
	if len(licensePublicKey) != ed25519.PublicKeySize {
		licenseBuildStatus.SetText("کلید عمومی هنگام فعال‌سازی به‌صورت امن از سرور Hesora دریافت و برای بررسی آفلاین ذخیره می‌شود.")
	} else {
		licenseBuildStatus.SetText("کلید عمومی سرور برای بررسی لایسنس در این نصب ذخیره شده است.")
	}
	licenseBuildStatus.SetStyleSheet("color: #52647d; padding: 10px; background: #edf2f8; border-radius: 8px;")
	settingsPage.layout.AddWidget(licenseBuildStatus.QWidget)
	licenseTokenInput := qt.NewQLineEdit(settingsPage.widget)
	licenseTokenInput.SetPlaceholderText("کد لایسنس خریداری‌شده از hesora.ir")
	settingsPage.layout.AddWidget(licenseTokenInput.QWidget)
	activateLicense := qt.NewQPushButton5("فعال‌سازی لایسنس پرو", settingsPage.widget)
	settingsPage.layout.AddWidget(activateLicense.QWidget)
	licenseActivationStatus := qt.NewQLabel5("", settingsPage.widget)
	licenseActivationStatus.SetWordWrap(true)
	settingsPage.layout.AddWidget(licenseActivationStatus.QWidget)
	connectCloudWithLicense := addCloudAndBackupSettings(settingsPage.widget, settingsPage.layout, store, licenseFiles, installationID, func() {
		store.SetProEnabled(false)
		templates.SetProEnabled(false)
		licenseState.SetText("Hesora رایگان")
	})
	licenseExpiryTimer := qt.NewQTimer2(settingsPage.widget.QObject)
	licenseExpiryTimer.OnTimeout(func() {
		token, _ := licenseFiles.ReadToken()
		result := verifier.Verify(token)
		store.SetProEnabled(result.HasFeature(licensing.ProFullFeature))
		templates.SetProEnabled(store.ProEnabled())
		if !store.ProEnabled() {
			licenseState.SetText("Hesora رایگان")
		} else if result.Claims != nil && result.Claims.ExpiresAt > 0 {
			licenseState.SetText(fmt.Sprintf("Hesora Pro — اعتبار باقی‌مانده: %d روز", (result.Claims.ExpiresAt-time.Now().Unix()+86399)/86400))
		}
	})
	licenseExpiryTimer.Start(60000)

	settingsPage.layout.AddWidget(sectionHeading(settingsPage.widget, "مقایسهٔ نسخه رایگان و پرو"))
	comparisonRows := [][3]string{
		{"امکانات", "رایگان", "پرو (اشتراکی یا دائمی)"},
		{"فروشگاه و ظرفیت", "۱ فروشگاه · ۵۰ مشتری · ۲۰۰ فاکتور", "استفادهٔ نامحدود و دائمی"},
		{"دسترسی وب و همگام‌سازی", "ندارد", "همهٔ امکانات در نسخهٔ وب؛ فاکتورها تا ۲ سال"},
		{"اعلان‌ها و به‌روزرسانی‌ها", "بدون اعلان و دریافت خودکار", "اعلان و دریافت آخرین تغییرات و نسخه‌ها"},
		{"پشتیبان‌گیری ابری", "پشتیبان محلی", "پشتیبان‌گیری خودکار و همیشه‌در‌دسترس"},
	}
	for rowIndex, values := range comparisonRows {
		row := qt.NewQHBoxLayout2()
		for _, value := range values {
			cell := qt.NewQLabel5(value, settingsPage.widget)
			cell.SetWordWrap(true)
			cell.SetMinimumWidth(200)
			if rowIndex == 0 {
				cell.SetStyleSheet("font-weight:700; color:#155eef; padding:8px;")
			} else {
				cell.SetStyleSheet("padding:8px; border-bottom:1px solid #e1e8f0;")
			}
			row.AddWidget(cell.QWidget)
		}
		settingsPage.layout.AddLayout(row.QLayout)
	}
	proPurchaseStatus := qt.NewQLabel5("برای خرید اشتراک یا لایسنس دائمی پرو به وب‌سایت Hesora مراجعه کنید.", settingsPage.widget)
	proPurchaseStatus.SetWordWrap(true)
	settingsPage.layout.AddWidget(proPurchaseStatus.QWidget)
	proPurchase := qt.NewQPushButton5("خرید و دریافت لایسنس پرو", settingsPage.widget)
	setInvoiceObjectName(proPurchase, "primaryAction")
	proPurchase.OnClicked(func() {
		if err := osExec.Command("rundll32.exe", "url.dll,FileProtocolHandler", "https://hesora.ir").Start(); err != nil {
			proPurchaseStatus.SetText("بازکردن وب‌سایت ناموفق بود؛ نشانی hesora.ir را در مرورگر وارد کنید.")
			return
		}
		proPurchaseStatus.SetText("وب‌سایت Hesora باز شد. مسیر و سامانهٔ خرید پس از راه‌اندازی سرویس در دسترس خواهد بود.")
	})
	settingsPage.layout.AddWidget(proPurchase.QWidget)

	settingsStatus := qt.NewQLabel5("تنظیمات به‌صورت محلی ذخیره می‌شوند.", settingsPage.widget)
	settingsStatus.SetWordWrap(true)
	settingsPage.layout.AddWidget(settingsStatus.QWidget)
	loadSetting := func(key string) string {
		setting, err := contacts.Setting(ctx, key)
		if err != nil {
			return ""
		}
		return setting.Value
	}
	fontName := loadSetting("font_family")
	if fontName == "" {
		fontName = "IRANSansX"
	}
	fontIndex := map[string]int{"IRANSansX": 0, "Tahoma (سیستم)": 1, "IRANSharp": 2, "IRAN Family Typeface": 3, "IRAN Kharazmi": 4, "IRANMarker": 5, "Segoe UI (سیستم)": 6}
	if i, ok := fontIndex[fontName]; ok {
		fontSetting.SetCurrentIndex(i)
	}
	fontSize := 10
	if n, err := strconv.Atoi(loadSetting("font_size")); err == nil && n >= 8 && n <= 32 {
		fontSize = n
	}
	fontSizeSetting.SetValue(fontSize)
	if _, ok := fontIndex[fontName]; !ok {
		fontSetting.SetCurrentIndex(0)
	}
	if loadSetting("theme") == "dark" {
		themeSetting.SetCurrentIndex(1)
	} else if loadSetting("theme") == "system" {
		themeSetting.SetCurrentIndex(2)
	}
	currency := loadSetting("default_currency")
	if currency == "" {
		currency = "IRT"
	}
	if currency == "IRR" {
		currencySetting.SetCurrentIndex(1)
	}

	fontFamilyPaths := map[string]string{
		"IRANSansX":            "assets/font/iransans/IRANSansX-Regular.ttf",
		"IRANSharp":            "assets/font/IRANSharp/IRANSharpRegular.ttf",
		"IRAN Family Typeface": "assets/font/IRAN Family Typeface/IRAN(FaNum).ttf",
		"IRAN Kharazmi":        "assets/font/IRAN_Kharazmi/IRAN Kharazmi.ttf",
		"IRANMarker":           "assets/font/iranmarker/IRANMarker.ttf",
	}
	resolveAssetPath := func(relativePath string) string {
		if _, err := os.Stat(relativePath); err == nil {
			return relativePath
		}
		if executable, err := os.Executable(); err == nil {
			candidate := filepath.Join(filepath.Dir(executable), relativePath)
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
		return relativePath
	}
	loadBundledFont := func(family string) string {
		path := fontFamilyPaths[family]
		if path == "" {
			return family
		}
		path = resolveAssetPath(path)
		if _, err := os.Stat(path); err != nil {
			return family
		}
		id := qt.QFontDatabase_AddApplicationFont(path)
		if id >= 0 {
			families := qt.QFontDatabase_ApplicationFontFamilies(id)
			if len(families) > 0 {
				return families[0]
			}
		}
		return family
	}
	applyAppearance := func() {
		family := fontSetting.CurrentText()
		family = loadBundledFont(family)
		if family == "Segoe UI (سیستم)" {
			family = "Segoe UI"
		}
		if family == "Tahoma (سیستم)" {
			family = "Tahoma"
		}
		font := qt.NewQFont6(family, fontSizeSetting.Value())
		qt.QApplication_SetFont(font)
		if themeSetting.CurrentIndex() == 1 {
			window.QWidget.SetStyleSheet(darkAppStyle(family, fontSizeSetting.Value()))
		} else {
			window.QWidget.SetStyleSheet(lightAppStyle(family, fontSizeSetting.Value()))
		}
	}
	type activationResult struct {
		key     ed25519.PublicKey
		license cloud.License
		err     error
	}
	activationResults := make(chan activationResult, 1)
	activationTimer := qt.NewQTimer2(settingsPage.widget.QObject)
	activationTimer.OnTimeout(func() {
		select {
		case result := <-activationResults:
			activationTimer.Stop()
			activateLicense.SetEnabled(true)
			if result.err != nil {
				licenseActivationStatus.SetText("فعال‌سازی انجام نشد: " + result.err.Error())
				return
			}
			newVerifier := licensing.Verifier{PublicKey: result.key, InstallationID: installationID, FreeFeatures: licensing.DefaultFreeFeatures()}
			verified := newVerifier.Verify(result.license.Token)
			if !verified.HasFeature(licensing.ProFullFeature) {
				if verified.Reason != nil {
					licenseActivationStatus.SetText("تأیید لایسنس دریافتی ناموفق بود: " + verified.Reason.Error())
				} else {
					licenseActivationStatus.SetText("کد واردشده دسترسی Pro ندارد.")
				}
				return
			}
			if err := licenseFiles.SavePublicKey(result.key); err != nil {
				licenseActivationStatus.SetText("ذخیرهٔ کلید عمومی ناموفق بود: " + err.Error())
				return
			}
			if err := licenseFiles.SaveToken(result.license.Token); err != nil {
				licenseActivationStatus.SetText("ذخیرهٔ لایسنس ناموفق بود: " + err.Error())
				return
			}
			licensePublicKey = result.key
			verifier = newVerifier
			licenseTokenInput.SetText("")
			store.SetProEnabled(true)
			templates.SetProEnabled(true)
			licenseState.SetText("Hesora Pro فعال")
			licenseDescription.SetText("لایسنس پرو فعال است. برای بکاپ ابری و مشاهدهٔ حساب، به حساب وب خود وارد شوید.")
			for _, choice := range proFontChoices {
				fontSetting.AddItem(choice)
			}
			applyAppearance()
			licenseActivationStatus.SetText("لایسنس فعال شد و به‌صورت خودکار به همین نصب متصل شد.")
			connectCloudWithLicense(result.license.Token)
		default:
		}
	})
	activateLicense.OnClicked(func() {
		if installationErr != nil {
			licenseActivationStatus.SetText("شناسهٔ داخلی نصب ساخته نشد: " + installationErr.Error())
			return
		}
		code := strings.TrimSpace(licenseTokenInput.Text())
		if code == "" {
			licenseActivationStatus.SetText("کد لایسنس خریداری‌شده را وارد کنید.")
			return
		}
		activateLicense.SetEnabled(false)
		licenseActivationStatus.SetText("در حال دریافت کلید عمومی و فعال‌سازی لایسنس از Hesora…")
		activationTimer.Start(100)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			client, err := cloud.New(hesoraCloudURL)
			if err != nil {
				activationResults <- activationResult{err: err}
				return
			}
			key, err := client.PublicKey(ctx)
			if err != nil {
				activationResults <- activationResult{err: err}
				return
			}
			license, err := client.ActivateLicense(ctx, code, installationID)
			activationResults <- activationResult{key: key, license: license, err: err}
		}()
	})

	saveSettings := qt.NewQPushButton5("ذخیره تنظیمات", settingsPage.widget)
	saveSettings.OnClicked(func() {
		selectedCurrency := "IRT"
		if currencySetting.CurrentIndex() == 1 {
			selectedCurrency = "IRR"
		}
		selectedTheme := "light"
		if themeSetting.CurrentIndex() == 1 {
			selectedTheme = "dark"
		} else if themeSetting.CurrentIndex() == 2 {
			selectedTheme = "system"
		}
		items := []domain.Setting{
			{Key: "default_currency", Value: selectedCurrency},
			{Key: "theme", Value: selectedTheme},
			{Key: "font_family", Value: fontSetting.CurrentText()},
			{Key: "font_size", Value: strconv.Itoa(fontSizeSetting.Value())},
		}
		for _, item := range items {
			if err := contacts.SetSetting(ctx, item); err != nil {
				settingsStatus.SetText("ذخیره تنظیمات انجام نشد: " + err.Error())
				return
			}
		}
		applyAppearance()
		settingsStatus.SetText("تنظیمات ذخیره و روی برنامه اعمال شد.")
	})
	settingsPage.layout.AddWidget(saveSettings.QWidget)
	settingsPage.layout.AddStretch()
	applyAppearance()

	tabs.AddTab(dashboard.widget, "داشبورد")
	tabs.AddTab(invoicePage.widget, "صدور فاکتور")
	tabs.AddTab(invoicesPage.widget, "فاکتورها")
	tabs.AddTab(customerPage.widget, "مشتریان")
	tabs.AddTab(issuerPage.widget, "فروشگاه")
	tabs.AddTab(settingsScroll.QWidget, "تنظیمات")
	tabs.OnCurrentChanged(func(index int) {
		if index == 0 {
			refreshDashboard()
		}
	})
	refreshDashboard()
	// Desktop shell: a permanent RTL sidebar keeps all major areas one click away.
	tabs.TabBar().Hide()
	shell := qt.NewQWidget(window.QWidget)
	shellLayout := qt.NewQHBoxLayout2()
	shellLayout.SetContentsMargins(0, 0, 0, 0)
	shellLayout.SetSpacing(0)
	shell.SetLayout(shellLayout.QLayout)

	sidebar := qt.NewQWidget(shell)
	setInvoiceObjectName(sidebar, "appSidebar")
	sidebar.SetFixedWidth(224)
	sidebarLayout := qt.NewQVBoxLayout2()
	sidebarLayout.SetContentsMargins(15, 20, 15, 16)
	sidebarLayout.SetSpacing(7)
	sidebar.SetLayout(sidebarLayout.QLayout)
	brandRow := qt.NewQHBoxLayout2()
	brandMark := qt.NewQLabel5("هـ", sidebar)
	setInvoiceObjectName(brandMark, "brandMark")
	brandRow.AddWidget(brandMark.QWidget)
	brandTitle := qt.NewQLabel5("هسورا\nمدیریت فروش", sidebar)
	setInvoiceObjectName(brandTitle, "brandTitle")
	brandRow.AddWidget(brandTitle.QWidget)
	brandRow.AddStretch()
	sidebarLayout.AddLayout(brandRow.QLayout)
	sidebarLayout.AddSpacing(24)
	caption := qt.NewQLabel5("فضای کاری", sidebar)
	setInvoiceObjectName(caption, "sidebarCaption")
	sidebarLayout.AddWidget(caption.QWidget)

	navItems := []struct {
		title string
		icon  string
		index int
	}{{"نمای کلی", "⌂", 0}, {"صدور فاکتور", "+", 1}, {"فاکتورها و قالب‌ها", "▤", 2}, {"مشتریان", "♙", 3}, {"فروشگاه و اطلاعات بانکی", "⌂", 4}, {"تنظیمات و لایسنس", "⚙", 5}}
	navButtons := make([]*qt.QPushButton, 0, len(navItems))
	for _, item := range navItems {
		button := qt.NewQPushButton5(item.icon+"   "+item.title, sidebar)
		setInvoiceObjectName(button, "navItem")
		button.SetCheckable(true)
		index := item.index
		button.OnClicked(func() {
			tabs.SetCurrentIndex(index)
			for i, navButton := range navButtons {
				navButton.SetChecked(i == index)
			}
		})
		navButtons = append(navButtons, button)
		sidebarLayout.AddWidget(button.QWidget)
	}
	sidebarLayout.AddStretch()
	sidebarFooter := qt.NewQLabel5("نسخهٔ رایگان هسورا\nتنظیمات و پشتیبانی", sidebar)
	sidebarFooter.SetWordWrap(true)
	setInvoiceObjectName(sidebarFooter, "sidebarCaption")
	sidebarLayout.AddWidget(sidebarFooter.QWidget)
	navButtons[0].SetChecked(true)

	content := qt.NewQWidget(shell)
	setInvoiceObjectName(content, "appContent")
	contentLayout := qt.NewQVBoxLayout2()
	contentLayout.SetContentsMargins(26, 18, 26, 22)
	contentLayout.SetSpacing(18)
	content.SetLayout(contentLayout.QLayout)
	topbar := qt.NewQWidget(content)
	setInvoiceObjectName(topbar, "appTopbar")
	topbarLayout := qt.NewQHBoxLayout2()
	topbarLayout.SetContentsMargins(16, 10, 16, 10)
	topbarLayout.SetSpacing(12)
	topbar.SetLayout(topbarLayout.QLayout)
	search := qt.NewQLineEdit(topbar)
	search.SetPlaceholderText("جست‌وجو در فاکتورها و مشتریان…")
	search.SetMaximumWidth(390)
	setInvoiceObjectName(search, "quickSearch")
	topbarLayout.AddWidget(search.QWidget)
	search.OnReturnPressed(func() {
		query := strings.TrimSpace(search.Text())
		switch {
		case strings.Contains(query, "مشتری"):
			tabs.SetCurrentIndex(3)
		case strings.Contains(query, "فروشگاه"), strings.Contains(query, "بانک"):
			tabs.SetCurrentIndex(4)
		case strings.Contains(query, "تنظیم"), strings.Contains(query, "لایسنس"):
			tabs.SetCurrentIndex(5)
		case strings.Contains(query, "صدور"), strings.Contains(query, "جدید"):
			tabs.SetCurrentIndex(1)
		default:
			tabs.SetCurrentIndex(2)
		}
		search.Clear()
	})
	topbarLayout.AddStretch()
	topbarCreate := qt.NewQPushButton5("＋ فاکتور جدید", topbar)
	topbarCreate.OnClicked(func() { tabs.SetCurrentIndex(1) })
	topbarLayout.AddWidget(topbarCreate.QWidget)
	contentLayout.AddWidget(topbar)
	contentLayout.AddWidget(tabs.QWidget)
	shellLayout.AddWidget(sidebar)
	shellLayout.AddWidget(content)
	window.SetCentralWidget(shell)
	window.Show()
	qt.QApplication_Exec()
}

type page struct {
	widget *qt.QWidget
	layout *qt.QVBoxLayout
}

func newPage(parent *qt.QWidget, title, subtitle string) page {
	w := qt.NewQWidget(parent)
	l := qt.NewQVBoxLayout2()
	l.SetContentsMargins(24, 22, 24, 22)
	l.SetSpacing(14)
	w.SetLayout(l.QLayout)
	h := qt.NewQLabel5(title, w)
	setInvoiceObjectName(h, "pageTitle")
	l.AddWidget(h.QWidget)
	s := qt.NewQLabel5(subtitle, w)
	s.SetWordWrap(true)
	setInvoiceObjectName(s, "muted")
	l.AddWidget(s.QWidget)
	return page{widget: w, layout: l}
}

func sectionHeading(parent *qt.QWidget, text string) *qt.QWidget {
	label := qt.NewQLabel5(text, parent)
	setInvoiceObjectName(label, "sectionTitle")
	return label.QWidget
}

func addField(layout *qt.QHBoxLayout, parent *qt.QWidget, label, placeholder string) {
	field := qt.NewQWidget(parent)
	fieldLayout := qt.NewQVBoxLayout2()
	fieldLayout.SetContentsMargins(0, 0, 0, 0)
	fieldLayout.SetSpacing(6)
	field.SetLayout(fieldLayout.QLayout)
	caption := qt.NewQLabel5(label, field)
	caption.SetStyleSheet("font-weight: 600; color: #425875;")
	input := qt.NewQLineEdit(field)
	input.SetPlaceholderText(placeholder)
	fieldLayout.AddWidget(caption.QWidget)
	fieldLayout.AddWidget(input.QWidget)
	layout.AddWidget(field)
}

func lightAppStyle(fontFamily string, fontSize int) string {
	return fmt.Sprintf(`QMainWindow, QWidget { background: #f4f7fb; color: #172b4d; font-family: "%s"; font-size: %dpt; }
QTabWidget::pane { border: 0; background: #f3f5f8; top: -1px; }
QTabBar::tab { background: #e9eff7; color: #52647d; border: 0; border-radius: 8px; padding: 12px 18px; margin: 5px 3px; min-width: 90px; font-weight: 600; }
QTabBar::tab:selected { background: #7058ed; color: white; }
QLineEdit, QComboBox, QDateEdit, QSpinBox { background: white; border: 1px solid #d6dfeb; border-radius: 7px; padding: 10px 12px; min-height: 20px; selection-background-color: #155eef; }
QLineEdit:focus, QComboBox:focus { border: 1px solid #7058ed; }
QPushButton { background: #7058ed; color: white; border: 0; border-radius: 8px; padding: 10px 16px; min-height: 20px; font-weight: 600; }
QPushButton:hover { background: #604bd8; }
QPushButton:pressed { background: #5140c5; }`+workspaceStyle(false)+shellStyle(false), fontFamily, fontSize)
}

func darkAppStyle(fontFamily string, fontSize int) string {
	return fmt.Sprintf(`QMainWindow, QWidget { background: #111827; color: #e5e7eb; font-family: "%s"; font-size: %dpt; }
QTabWidget::pane { border: 1px solid #374151; border-radius: 12px; background: #111827; top: -1px; }
QTabBar::tab { background: #1f2937; color: #cbd5e1; border: 0; border-radius: 8px; padding: 12px 18px; margin: 5px 3px; min-width: 90px; font-weight: 600; }
QTabBar::tab:selected { background: #7058ed; color: white; }
QLineEdit, QComboBox, QDateEdit, QSpinBox { background: #1f2937; color: #f9fafb; border: 1px solid #4b5563; border-radius: 7px; padding: 10px 12px; min-height: 20px; selection-background-color: #2563eb; }
QPushButton { background: #7058ed; color: white; border: 0; border-radius: 8px; padding: 10px 16px; min-height: 20px; font-weight: 600; }
QPushButton:hover { background: #604bd8; }
QLabel#muted { color: #9ca3af; }`+workspaceStyle(true)+shellStyle(true), fontFamily, fontSize)
}

func shellStyle(dark bool) string {
	if dark {
		return `QWidget#appSidebar{background:#192631;border:0;} QLabel#brandMark{background:#7058ed;color:white;border-radius:12px;font-size:19pt;font-weight:700;padding:9px;} QLabel#brandTitle{background:transparent;color:white;font-size:14pt;font-weight:700;} QLabel#sidebarCaption{background:transparent;color:#8997a4;font-size:9pt;padding:8px 10px;} QPushButton#navItem{background:transparent;color:#b9c4cf;border:0;border-radius:9px;text-align:right;padding:12px 14px;min-height:24px;} QPushButton#navItem:hover{background:#253440;color:white;} QPushButton#navItem:checked{background:#2c3946;color:white;font-weight:700;} QWidget#appTopbar{background:#1f2937;border:1px solid #374151;border-radius:12px;} QLineEdit#quickSearch{background:#263444;border:0;color:#e5e7eb;} QWidget#appContent{background:#111827;} QTabWidget::pane{background:#111827;}`
	}
	return `QWidget#appSidebar{background:#192631;border:0;} QLabel#brandMark{background:#7058ed;color:white;border-radius:12px;font-size:19pt;font-weight:700;padding:9px;} QLabel#brandTitle{background:transparent;color:white;font-size:14pt;font-weight:700;} QLabel#sidebarCaption{background:transparent;color:#8997a4;font-size:9pt;padding:8px 10px;} QPushButton#navItem{background:transparent;color:#b9c4cf;border:0;border-radius:9px;text-align:right;padding:12px 14px;min-height:24px;} QPushButton#navItem:hover{background:#253440;color:white;} QPushButton#navItem:checked{background:#2c3946;color:white;font-weight:700;} QWidget#appTopbar{background:white;border:1px solid #e5e9ef;border-radius:12px;} QLineEdit#quickSearch{background:#f5f6f9;border:0;} QWidget#appContent{background:#f3f5f8;} QTabWidget::pane{background:#f3f5f8;}`
}

// resolveApplicationIconPath finds the product-owned icon beside the process
// working directory or executable. The icon is bundled with the application,
// not selected or persisted by the user.
func resolveApplicationIconPath() string {
	const relative = "assets/invoice-maker.ico"
	if _, err := os.Stat(relative); err == nil {
		return relative
	}
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), relative)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return relative
}

// Shared surfaces inherit the selected application theme.
func workspaceStyle(dark bool) string {
	background, border, muted, title, soft := "#ffffff", "#dfe7f1", "#64748b", "#17345f", "#edf3fb"
	if dark {
		background, border, muted, title, soft = "#1f2937", "#374151", "#a6b4c8", "#e5efff", "#26364c"
	}
	return fmt.Sprintf(`
QScrollArea { border: 0; background: transparent; }
QWidget#surface { background: %s; border: 1px solid %s; border-radius: 14px; }
QLabel, QCheckBox { background: transparent; }
QWidget#fieldGroup, QWidget#itemRow { background: transparent; }
QWidget#surface QLineEdit, QWidget#surface QComboBox, QWidget#surface QPlainTextEdit { background: %s; }
QLabel#pageTitle { color: %s; font-size: 19pt; font-weight: 700; }
QLabel#sectionTitle { color: %s; font-size: 11pt; font-weight: 700; }
QLabel#muted { color: %s; }
QLabel#metricValue { color: %s; font-size: 17pt; font-weight: 700; }
QPushButton#primaryAction { background: #7058ed; color: white; }
QLabel#grandTotal { background: %s; color: %s; padding: 18px; border-radius: 10px; font-size: 15pt; font-weight: 700; }
QPushButton#secondaryAction { background: %s; color: %s; border: 1px solid %s; }
QPushButton#removeItem { background: %s; color: %s; border: 1px solid %s; padding: 8px; }
QPushButton:disabled { background: %s; color: %s; }
QPlainTextEdit { border: 1px solid %s; border-radius: 8px; padding: 10px; }
QScrollBar:vertical { background: transparent; width: 10px; margin: 0; }
QScrollBar:horizontal { background: transparent; height: 10px; margin: 0; }
QScrollBar::handle:vertical { background: #718198; min-height: 28px; border-radius: 5px; }
QScrollBar::handle:horizontal { background: #718198; min-width: 28px; border-radius: 5px; }
QScrollBar::add-line, QScrollBar::sub-line { width: 0; height: 0; }
QScrollBar::add-page, QScrollBar::sub-page { background: transparent; }
`, background, border, background, title, title, muted, title, soft, title, soft, title, border, soft, muted, border, soft, muted, border)
}
