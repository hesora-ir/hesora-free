# نمونه اولیه Go و Qt 6

## هدف
بررسی حداقلی اتصال Go به Qt Widgets، ساخت پنجره فارسی، تنظیم جهت RTL و دریافت رویداد کلیک. این نمونه صرفاً آزمایش فناوری است و قابلیت محصول/فاکتور ندارد.

## Binding آزمایشی
MIQT، ماژول `github.com/mappu/miqt` نسخه `v0.15.0` با import مسیر `github.com/mappu/miqt/qt6`.
- Binding تحت MIT است.
- MIQT از CGO و toolchain C++ سازگار با Qt استفاده می‌کند.
- مجوز Qt و شرایط توزیع DLLهای آن جداگانه باید رعایت شود.
- سازگاری دقیق نسخه Qt 6.12.0 با محیط محلی هنوز با Build واقعی تأیید نشده است.

## فایل نمونه
کد در `cmd/qt-prototype/main.go` است. پنجره، متن فارسی، جهت RTL و دکمه‌ای با شمارنده رویداد را ایجاد می‌کند.

## پیش‌نیازهای Windows
1. Go 1.25.1
2. Qt 6.12.0 برای معماری x86_64 و بسته MinGW سازگار با همان نصب
3. GCC/G++ یا toolchain دقیقاً مطابق بسته Qt
4. `pkg-config` و فایل‌های `.pc` مربوط به Qt
5. متغیرهای محیطی CGO و مسیر کتابخانه‌ها

توجه: نصب Go و Qt به‌تنهایی کافی نیست. MSVC با CGO در راهنمای MIQT به‌عنوان toolchain سازگار معرفی نشده؛ compiler باید با toolchain Qt منطبق باشد.

## چک‌لیست PowerShell
مقادیر مسیرها را با نصب واقعی تطبیق دهید؛ این فرمان‌ها در محیط توسعه پروژه اجرا نشده‌اند:

    go version
    go env CGO_ENABLED GOOS GOARCH
    where.exe go
    where.exe gcc
    where.exe g++
    where.exe pkg-config
    pkg-config --modversion Qt6Widgets

در صورت نیاز، متغیرهای محیطی را فقط برای همان نشست PowerShell تنظیم کنید. مسیر دقیق MinGW و pkg-config باید از نصب واقعی Qt استخراج شود؛ مسیر فرضی را به‌عنوان مسیر معتبر وارد نکنید.

پس از تنظیم محیط:

    go mod download
    go mod tidy
    gofmt -w cmd/qt-prototype/main.go
    go test ./...
    go build -v -o dist/qt-prototype.exe ./cmd/qt-prototype

اجرای نمونه:

    .\dist\qt-prototype.exe

اگر از Qt به‌صورت Dynamic استفاده می‌شود، DLLهای Qt و platform plugin مناسب (معمولاً پوشه platforms با افزونه qwindows) باید در زمان اجرا در دسترس باشند. بسته‌بندی این وابستگی‌ها در فاز Installer انجام می‌شود.

## معیار پذیرش
- [ ] ثبت خروجی نسخه واقعی Go، Qt، MinGW، CMake، Ninja و WiX
- [ ] `pkg-config --modversion Qt6Widgets` نسخه نصب‌شده را نشان دهد
- [ ] `go test ./...` موفق باشد
- [ ] فایل اجرایی بدون خطای DLL اجرا شود
- [ ] پنجره فارسی نمایش داده شود و جهت RTL درست باشد
- [ ] کلیک دکمه شمارنده را تغییر دهد
- [ ] Build روی محیط تمیز تکرار شود
- [ ] نسخه، مجوز و محدودیت‌ها در مستندات ثبت شوند

## وضعیت اجرا
در محیط فعلی امکان اجرای Windows و toolchain Qt نصب‌شده در مسیر G:\qt\6.12.0\mingw_64 وجود نداشت. بنابراین نمونه در این تغییر به مخزن افزوده شده، اما هنوز Build، اجرا یا تست واقعی آن گزارش نمی‌شود. پذیرش فاز ۱ تا اجرای موفق روی Windows هدف کامل نیست.
