# ساخت و اجرا

## اجرای سریع توسعه، بدون نصب

از ریشهٔ مخزن در PowerShell اجرا کنید:

```powershell
.\dev.ps1
```

اسکریپت `dev.ps1` ابتدا ابزارهای Qt/MinGW را در همین نشست تنظیم و بررسی می‌کند، سپس `go test ./...` و `go vet ./...` را اجرا و برنامه را با Go می‌سازد. DLLها و pluginهای لازم با windeployqt و فونت‌ها/آیکون‌ها از assets کنار خروجی قرار می‌گیرند. در پایان پنجرهٔ برنامه باز می‌شود؛ MSI، WiX، CMake و نصب مجدد برای این مسیر لازم نیستند.

مسیرهای پیش‌فرض این سیستم:

| ابزار | مسیر |
|---|---|
| Qt | `G:\qt\6.12.0\mingw_64` |
| MinGW | `G:\qt\Tools\mingw1310_64` |
| pkg-config | از PATH؛ در این سیستم `C:\msys64\mingw64\bin\pkg-config.exe` |
| خروجی | `build\dev\bin\invoice-maker.exe` |

اسکریپت `QT_ROOT`، `PKG_CONFIG_PATH`، `PKG_CONFIG`، `CGO_ENABLED`، `CC`، `CXX`، `GOMAXPROCS`، مسیر pluginهای Qt و PATH همین نشست را تنظیم می‌کند؛ تنظیم سراسری ویندوز یا `go env -w` انجام نمی‌دهد. مسیر Go انتخاب‌شده در ابتدای PATH نگه داشته می‌شود تا ابزار Go همراه MSYS2 جای آن را نگیرد. تغییرات PATH به برنامه‌های بازشدهٔ قبلی یا PowerShell دیگر منتقل نمی‌شوند.

بعد از تغییرات، برای build و اجرا بدون تکرار تست‌ها:

```powershell
.\dev.ps1 -SkipChecks
```

اگر نسخهٔ توسعه هنوز باز است، پنجره را ببندید و دوباره اجرا کنید، یا برای درخواست بستن عادی همان پنجره:

```powershell
.\dev.ps1 -Restart -SkipChecks
```

`-Restart` فقط executable همین مسیر توسعه را بررسی می‌کند؛ نسخه‌های نصب‌شده را نمی‌بندد و هیچ پردازشی را به اجبار terminate نمی‌کند. اگر برنامه عادی بسته نشود، build متوقف می‌شود. پیش از restart ویرایش‌های خود را ذخیره کنید.

برای تست و build بدون باز شدن پنجره:

```powershell
.\dev.ps1 -NoRun
```

برای تنظیم محیط و سپس اجرای مستقیم فرمان‌های Go در همین PowerShell، فرمان نخست را با **dot-source** اجرا کنید:

```powershell
. .\dev.ps1 -SetupOnly
go test ./...
go vet ./...
```

برای مسیر سفارشی ابزارها:

```powershell
.\dev.ps1 -QtRoot 'G:\qt\6.12.0\mingw_64' -MingwRoot 'G:\qt\Tools\mingw1310_64' -PkgConfig 'C:\msys64\mingw64\bin\pkg-config.exe'
```

اگر `QT_ROOT` از قبل در محیط وجود داشته باشد، پیش‌فرض از همان خوانده می‌شود؛ `-QtRoot` همیشه صریحاً آن را عوض می‌کند. kit باید Qt 6 ویندوز x64 از نوع MinGW باشد، نه Android یا MSVC. وجود فایل‌های Qt6Widgets.pc، Qt6Pdf.pc و Qt6PdfWidgets.pc و دسترسی pkg-config به آن‌ها بررسی می‌شود. Go و pkg-config باید از قبل نصب باشند؛ اسکریپت نرم‌افزاری نصب نمی‌کند.

cache استاندارد Go حفظ می‌شود و فایل‌های assets فقط هنگام تغییر کپی می‌شوند. تعداد کارهای هم‌زمان پیش‌فرض ۲ است تا کامپایل bindingهای Qt با کمبود حافظه مواجه نشود؛ با `-Jobs 4` می‌توانید آن را تغییر دهید. اولین اجرای cache خالی یا تغییر کامپایلر ممکن است طولانی باشد؛ اجراهای بعدی از cache استفاده می‌کنند. برای فعال‌سازی Pro در build توسعه، همان متغیر `HESORA_LICENSE_PUBLIC_KEY` معرفی‌شده در راهنمای اتصال وب قابل استفاده است. اجرای توسعه از محل معمول دادهٔ برنامه، یعنی `%LOCALAPPDATA%\invoice-maker`، استفاده می‌کند و rebuild داده‌ها را پاک نمی‌کند. این خروجی برای مشاهدهٔ تغییرات روی همین دستگاه است؛ تأیید بستهٔ انتشار روی ویندوز تمیز جداگانه انجام می‌شود.

## پیش‌نیازها
- Go سازگار با نسخه پروژه؛ اسکریپت Installer فعلی Go 1.25.1 را بررسی می‌کند.
- Qt 6.12.0 MinGW در مسیر پیش‌فرض `G:\qt\6.12.0\mingw_64` یا مسیر معرفی‌شده با `-QtRoot` / متغیر `QT_ROOT`.
- WiX Toolset 6.0.2 و .NET SDK.
- محیط Windows x64.

## اجرای تست‌ها و بررسی کد
از ریشه مخزن:

```powershell
go test ./...
go vet ./...
gofmt -l .
```

برای اجرای مستقیم با Go، ممکن است لازم باشد متغیرهای محیط Qt/MinGW طبق `docs/DEVELOPMENT_SETUP.md` تنظیم شوند.

## ساخت برنامه بدون پنجره کنسول
برای برنامه GUI ویندوز باید subsystem گرافیکی در linker مشخص شود:

```powershell
go build -trimpath -ldflags="-H=windowsgui" -o .\artifacts\publish\invoice-maker.exe .\cmd\invoice-maker
```

اگر برنامه با `go build` عادی یا `go run` اجرا شود، ویندوز ممکن است پنجره کنسول جداگانه باز کند. اسکریپت Installer از فلگ بالا استفاده می‌کند.

## ساخت بسته Installer
فرمان زیر برنامه را می‌سازد، وابستگی‌های Qt را با `windeployqt` کپی می‌کند و سپس MSI را با WiX می‌سازد:

```powershell
.\scripts\build-installer.ps1
```

برای مشخص‌کردن مسیر دیگر Qt:

```powershell
.\scripts\build-installer.ps1 -QtRoot "G:\qt\6.12.0\mingw_64"
```

در صورت موفقیت، MSI در مسیر زیر قرار می‌گیرد:

```
artifacts\installer\InvoiceMakerSetup-0.1.0-x64.msi
```

## آزمون اولیه MSI

```powershell
.\scripts\test-installer.ps1 -MsiPath ".\artifacts\installer\InvoiceMakerSetup-0.1.0-x64.msi"
```

این بررسی اولیه جایگزین نصب واقعی روی Windows تمیز، آزمون ارتقا/حذف و تأیید حفظ دیتابیس کاربر نیست.

## وضعیت رابط کاربری
چیدمان فارسی و RTL و شش بخش اصلی در `docs/UI_LAYOUT.md` مستند شده‌اند. داشبورد و صفحات صدور/فاکتورها فعلاً scaffold نمایشی هستند؛ شمارنده‌های داشبورد عمداً داده ساختگی نشان نمی‌دهند. ثبت و جست‌وجوی مشتری، پروفایل صادرکننده و ذخیره تنظیمات به لایه Application/SQLite متصل‌اند.

## محدودیت اعتبارسنجی
این مستند فرمان‌ها و تنظیمات مورد انتظار را ثبت می‌کند، اما به‌تنهایی به معنی موفقیت Build یا تست روی Windows نیست. خروجی واقعی تست‌ها، اجرا و نصب باید جداگانه بررسی و ثبت شود.
