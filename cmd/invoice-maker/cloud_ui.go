package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hesora-ir/hesora-free/internal/adapters/sqlite"
	"github.com/hesora-ir/hesora-free/internal/cloud"
	"github.com/hesora-ir/hesora-free/internal/licensing"
	qt "github.com/mappu/miqt/qt6"
)

var hesoraCloudURL = "https://hesora.ir"

// All network and snapshot work runs outside the Qt event loop. Completion is
// consumed by a parent-owned timer, so widget updates stay on the UI thread.
func addCloudAndBackupSettings(parent *qt.QWidget, layout *qt.QVBoxLayout, store *sqlite.Store, files licensing.FileStore, installationID string, onRevoked func()) {
	layout.AddWidget(sectionHeading(parent, "بکاپ و اتصال به فضای ابری"))
	status := qt.NewQLabel5("بکاپ محلی بدون اشتراک در دسترس است. برای بکاپ ابری وارد حساب شوید.", parent)
	status.SetWordWrap(true)
	local := qt.NewQPushButton5("ذخیره بکاپ روی حافظه کامپیوتر", parent)
	layout.AddWidget(local.QWidget)
	urlInput := qt.NewQLineEdit(parent)
	urlInput.SetLayoutDirection(qt.LeftToRight)
	urlInput.SetText(hesoraCloudURL)
	urlInput.SetPlaceholderText("https://hesora.ir")
	layout.AddWidget(qt.NewQLabel5("نشانی سرویس ابری (فقط سرویس مورد اعتماد خود را وارد کنید)", parent).QWidget)
	layout.AddWidget(urlInput.QWidget)
	email := qt.NewQLineEdit(parent)
	email.SetLayoutDirection(qt.LeftToRight)
	email.SetPlaceholderText("ایمیل حساب ابری")
	layout.AddWidget(email.QWidget)
	password := qt.NewQLineEdit(parent)
	password.SetEchoMode(qt.QLineEdit__Password)
	password.SetPlaceholderText("رمز عبور حساب ابری")
	layout.AddWidget(password.QWidget)
	login := qt.NewQPushButton5("ورود به حساب ابری", parent)
	logout := qt.NewQPushButton5("خروج از حساب ابری", parent)
	upload := qt.NewQPushButton5("بکاپ‌گیری ابری از اطلاعات فعلی", parent)
	refresh := qt.NewQPushButton5("مشاهده حساب و فهرست بکاپ‌ها", parent)
	validate := qt.NewQPushButton5("بررسی آنلاین اعتبار لایسنس", parent)
	panel := qt.NewQPushButton5("ساخت حساب و مدیریت بکاپ‌ها در پنل وب", parent)
	panel.OnClicked(func() {
		if _, err := cloud.New(urlInput.Text()); err != nil {
			status.SetText(err.Error())
			return
		}
		if !qt.QDesktopServices_OpenUrl(qt.NewQUrl3(strings.TrimRight(strings.TrimSpace(urlInput.Text()), "/") + "/account")) {
			status.SetText("باز کردن مرورگر ناموفق بود؛ نشانی پنل را در مرورگر باز کنید.")
		}
	})
	layout.AddWidget(panel.QWidget)
	for _, b := range []*qt.QPushButton{login, logout, upload, refresh, validate} {
		layout.AddWidget(b.QWidget)
	}
	layout.AddWidget(status.QWidget)
	var client *cloud.Client
	type completion struct {
		apply func()
		err   error
	}
	results := make(chan completion, 1)
	busy := false
	buttons := []*qt.QPushButton{local, login, logout, upload, refresh, validate}
	timer := qt.NewQTimer2(parent.QObject)
	timer.OnTimeout(func() {
		select {
		case result := <-results:
			timer.Stop()
			busy = false
			for _, b := range buttons {
				b.SetEnabled(true)
			}
			if result.apply != nil {
				result.apply()
			}
			if result.err != nil {
				status.SetText("عملیات ناموفق: " + result.err.Error())
			}
		default:
		}
	})
	run := func(job func() (func(), error)) {
		if busy {
			return
		}
		busy = true
		for _, b := range buttons {
			b.SetEnabled(false)
		}
		status.SetText("در حال انجام عملیات…")
		timer.Start(100)
		go func() { apply, err := job(); results <- completion{apply, err} }()
	}
	local.OnClicked(func() {
		if busy {
			return
		}
		path := qt.QFileDialog_GetSaveFileName4(parent, "ذخیره بکاپ در فایل جدید", "hesora-"+time.Now().Format("20060102-150405")+".db", "SQLite Backup (*.db)")
		if path == "" {
			return
		}
		run(func() (func(), error) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			if err := store.BackupTo(ctx, path); err != nil {
				return nil, err
			}
			return func() { status.SetText("بکاپ سالم ذخیره شد: " + path) }, nil
		})
	})
	accountText := func(a cloud.Account) string {
		summary := fmt.Sprintf("حساب: %s\nبکاپ‌ها: %d | مصرف فضا: %d کیلوبایت", a.User.Name, a.BackupCount, a.StorageBytes/1024)
		for _, l := range a.Licenses {
			if l.InstallationID == installationID {
				remaining := "دائمی"
				if l.ExpiresAt > 0 {
					remaining = fmt.Sprintf("%d روز", (l.RemainingSeconds+86399)/86400)
				}
				summary += fmt.Sprintf("\nلایسنس این نصب: %s | اعتبار: %s", l.Status, remaining)
			}
		}
		return summary
	}
	login.OnClicked(func() {
		base, emailValue, passwordValue := urlInput.Text(), email.Text(), password.Text()
		password.SetText("")
		run(func() (func(), error) {
			next, err := cloud.New(base)
			if err != nil {
				return nil, err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			if err = next.Login(ctx, emailValue, passwordValue); err != nil {
				return nil, err
			}
			a, err := next.Account(ctx)
			if err != nil {
				return nil, err
			}
			return func() {
				client = next
				urlInput.SetEnabled(false)
				status.SetText("اتصال برقرار شد.\n" + accountText(a))
			}, nil
		})
	})
	requireClient := func() bool {
		if client == nil {
			status.SetText("ابتدا وارد حساب ابری شوید.")
			return false
		}
		return true
	}
	logout.OnClicked(func() {
		if !requireClient() {
			return
		}
		current := client
		run(func() (func(), error) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			err := current.Logout(ctx)
			return func() {
				client = nil
				urlInput.SetEnabled(true)
				status.SetText("از حساب ابری خارج شدید.")
			}, err
		})
	})
	refresh.OnClicked(func() {
		if !requireClient() {
			return
		}
		current := client
		run(func() (func(), error) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			a, err := current.Account(ctx)
			if err != nil {
				return nil, err
			}
			backups, err := current.Backups(ctx)
			if err != nil {
				return nil, err
			}
			text := accountText(a)
			for _, b := range backups {
				text += fmt.Sprintf("\n%s — %d کیلوبایت — %s", b.Name, b.Size/1024, time.Unix(b.CreatedAt, 0).Format("2006-01-02 15:04"))
			}
			return func() { status.SetText(text) }, nil
		})
	})
	upload.OnClicked(func() {
		if !requireClient() {
			return
		}
		current := client
		run(func() (func(), error) {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			dir, err := os.MkdirTemp(filepath.Dir(store.Path()), ".cloud-backup-")
			if err != nil {
				return nil, err
			}
			defer os.RemoveAll(dir)
			path := filepath.Join(dir, "hesora-"+time.Now().Format("20060102-150405")+".db")
			if err = store.BackupTo(ctx, path); err != nil {
				return nil, err
			}
			b, err := current.UploadBackup(ctx, path)
			if err != nil {
				return nil, err
			}
			return func() {
				status.SetText(fmt.Sprintf("بکاپ ابری ذخیره شد: %s\nشناسه: %s\nبرای دانلود و حذف بکاپ‌ها وارد پنل وب شوید.", b.Name, b.ID))
			}, nil
		})
	})
	validate.OnClicked(func() {
		if !requireClient() {
			return
		}
		token, err := files.ReadToken()
		if err != nil {
			status.SetText("لایسنس محلی فعال وجود ندارد.")
			return
		}
		current := client
		run(func() (func(), error) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			l, err := current.ValidateLicense(ctx, token, installationID)
			if err != nil {
				if cloud.IsForbidden(err) {
					return func() {
						files.ClearToken()
						onRevoked()
						status.SetText("لایسنس توسط سرور رد شد؛ نسخه رایگان فعال شد. اطلاعات شما محفوظ است.")
					}, nil
				}
				return nil, err
			}
			return func() { status.SetText("لایسنس آنلاین معتبر است. وضعیت: " + l.Status) }, nil
		})
	})
}
