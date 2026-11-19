package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hesora-ir/hesora-free/internal/adapters/sqlite"
	"github.com/hesora-ir/hesora-free/internal/cloud"
	"github.com/hesora-ir/hesora-free/internal/domain"
	"github.com/hesora-ir/hesora-free/internal/licensing"
	qt "github.com/mappu/miqt/qt6"
)

var activeCloudClient *cloud.Client

var hesoraCloudURL = "https://hesora.ir"

// All network and snapshot work runs outside the Qt event loop. Completion is
// consumed by a parent-owned timer, so widget updates stay on the UI thread.
func addCloudAndBackupSettings(parent *qt.QWidget, layout *qt.QVBoxLayout, store *sqlite.Store, files licensing.FileStore, installationID string, onRevoked func()) func(string) {
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
	login := qt.NewQPushButton5("ورود به حساب موجود (اختیاری)", parent)
	logout := qt.NewQPushButton5("خروج از حساب ابری", parent)
	logout.SetVisible(false)
	upload := qt.NewQPushButton5("بکاپ‌گیری ابری از اطلاعات فعلی", parent)
	syncWorkspace := qt.NewQPushButton5("همگام‌سازی مستقیم با فضای کار وب", parent)
	autoSync := qt.NewQCheckBox4("همگام‌سازی خودکار هر ۵ دقیقه (با تعارض، نسخهٔ دسکتاپ جایگزین نسخهٔ وب می‌شود)", parent)
	autoSync.SetToolTip("با فعال‌سازی، اجازه می‌دهید همگام‌سازی دوره‌ای سیاست دسکتاپ‌برنده را بدون پرسش دوباره اجرا کند.")
	if saved, err := store.GetSetting(context.Background(), "cloud.auto_sync"); err == nil && saved.Value == "true" {
		autoSync.SetChecked(true)
	}
	autoSyncTimer := qt.NewQTimer2(parent.QObject)
	autoSyncTimer.SetInterval(5 * 60 * 1000)
	refresh := qt.NewQPushButton5("مشاهده حساب و فهرست بکاپ‌ها", parent)
	backupID := qt.NewQLineEdit(parent)
	backupID.SetLayoutDirection(qt.LeftToRight)
	backupID.SetPlaceholderText("شناسه بکاپ برای دریافت (از فهرست پایین کپی کنید)")
	download := qt.NewQPushButton5("دانلود بکاپ انتخاب‌شده", parent)
	restoreBackup := qt.NewQPushButton5("بازیابی بکاپ در برنامه", parent)
	deleteBackup := qt.NewQPushButton5("حذف بکاپ انتخاب‌شده", parent)
	validate := qt.NewQPushButton5("بررسی آنلاین اعتبار لایسنس", parent)
	registerOnline := qt.NewQPushButton5("ساخت حساب فقط در وب‌سایت Hesora", parent)
	registerOnline.SetToolTip("ساخت حساب از داخل برنامهٔ دسکتاپ انجام نمی‌شود؛ ثبت‌نام را در وب‌سایت کامل کنید.")
	registerOnline.OnClicked(func() {
		if _, err := cloud.New(urlInput.Text()); err != nil {
			status.SetText(err.Error())
			return
		}
		if !qt.QDesktopServices_OpenUrl(qt.NewQUrl3(strings.TrimRight(strings.TrimSpace(urlInput.Text()), "/") + "/register")) {
			status.SetText("باز کردن صفحهٔ ثبت‌نام وب ناموفق بود؛ نشانی hesora.ir/register را در مرورگر باز کنید.")
		}
	})
	layout.AddWidget(registerOnline.QWidget)
	for _, b := range []*qt.QPushButton{login, logout, upload, syncWorkspace, refresh, validate} {
		layout.AddWidget(b.QWidget)
	}
	layout.AddWidget(autoSync.QWidget)
	layout.AddWidget(qt.NewQLabel5("برای دریافت یک بکاپ، شناسهٔ آن را از فهرست حساب وارد کنید:", parent).QWidget)
	layout.AddWidget(backupID.QWidget)
	layout.AddWidget(download.QWidget)
	layout.AddWidget(restoreBackup.QWidget)
	layout.AddWidget(deleteBackup.QWidget)
	layout.AddWidget(status.QWidget)
	var client *cloud.Client
	var startSync func(confirmDesktopWins bool)
	type completion struct {
		apply func()
		err   error
	}
	results := make(chan completion, 1)
	busy := false
	buttons := []*qt.QPushButton{local, login, logout, upload, syncWorkspace, refresh, download, restoreBackup, deleteBackup, validate}
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
		cloudStatus := "فعال"
		if !a.CloudEnabled {
			cloudStatus = "غیرفعال؛ از پشتیبانی بخواهید دسترسی ابری را فعال کند"
		}
		summary := fmt.Sprintf("حساب: %s\nسرویس ابری: %s\nبکاپ‌ها: %d | مصرف فضا: %d کیلوبایت", a.User.Name, cloudStatus, a.BackupCount, a.StorageBytes/1024)
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
	connectWithLicense := func(licenseToken string) {
		if busy || strings.TrimSpace(licenseToken) == "" {
			return
		}
		base := urlInput.Text()
		run(func() (func(), error) {
			next, err := cloud.New(base)
			if err != nil {
				return nil, err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := next.DeviceLogin(ctx, licenseToken, installationID); err != nil {
				return nil, err
			}
			a, err := next.Account(ctx)
			if err != nil {
				return nil, err
			}
			return func() {
				client = next
				activeCloudClient = next
				urlInput.SetEnabled(false)
				email.SetVisible(false)
				password.SetVisible(false)
				login.SetVisible(false)
				logout.SetVisible(true)
				status.SetText("اتصال با حساب خرید لایسنس برقرار شد.\n" + accountText(a))
				if autoSync.IsChecked() {
					startSync(false)
				}
			}, nil
		})
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
				activeCloudClient = next
				urlInput.SetEnabled(false)
				email.SetVisible(false)
				password.SetVisible(false)
				login.SetVisible(false)
				logout.SetVisible(true)
				status.SetText("اتصال برقرار شد.\n" + accountText(a))
				if autoSync.IsChecked() {
					startSync(false)
				}
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
				activeCloudClient = nil
				urlInput.SetEnabled(true)
				email.SetVisible(true)
				password.SetVisible(true)
				login.SetVisible(true)
				logout.SetVisible(false)
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
				text += fmt.Sprintf("\n%s — %d کیلوبایت — %s\nشناسه: %s", b.Name, b.Size/1024, time.Unix(b.CreatedAt, 0).Format("2006-01-02 15:04"), b.ID)
			}
			return func() { status.SetText(text) }, nil
		})
	})
	download.OnClicked(func() {
		if !requireClient() {
			return
		}
		id := strings.TrimSpace(backupID.Text())
		if id == "" {
			status.SetText("ابتدا شناسه بکاپ را از فهرست بکاپ‌ها وارد کنید.")
			return
		}
		path := qt.QFileDialog_GetSaveFileName4(parent, "ذخیره بکاپ ابری", "hesora-cloud-backup.db", "SQLite Backup (*.db)")
		if path == "" {
			return
		}
		if !strings.HasSuffix(strings.ToLower(path), ".db") {
			path += ".db"
		}
		current := client
		run(func() (func(), error) {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			if err := current.DownloadBackup(ctx, id, path); err != nil {
				return nil, err
			}
			return func() { status.SetText("بکاپ دانلود و صحت checksum آن بررسی شد:\n" + path) }, nil
		})
	})
	restoreBackup.OnClicked(func() {
		if !requireClient() {
			return
		}
		id := strings.TrimSpace(backupID.Text())
		if id == "" {
			status.SetText("ابتدا شناسهٔ بکاپ را از فهرست وارد کنید.")
			return
		}
		answer := qt.QMessageBox_Question2(parent, "بازیابی فضای کاری", "تمام اطلاعات محلی با نسخهٔ انتخاب‌شده جایگزین می‌شود. نسخهٔ فعلی برای بازگشت کنار پایگاه داده نگه داشته خواهد شد. ادامه می‌دهید؟", qt.QMessageBox__Yes, qt.QMessageBox__No)
		if answer != int(qt.QMessageBox__Yes) {
			return
		}
		current := client
		run(func() (func(), error) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			dir, err := os.MkdirTemp(filepath.Dir(store.Path()), ".cloud-restore-")
			if err != nil {
				return nil, err
			}
			defer os.RemoveAll(dir)
			path := filepath.Join(dir, "selected-backup.db")
			if err = current.DownloadBackup(ctx, id, path); err != nil {
				return nil, err
			}
			safety, err := store.RestoreFrom(ctx, path)
			if err != nil {
				return nil, err
			}
			return func() {
				status.SetText("بازیابی کامل شد. نسخهٔ پیش از بازیابی برای بازگشت در این مسیر نگه داشته شد:\n" + safety)
			}, nil
		})
	})
	deleteBackup.OnClicked(func() {
		if !requireClient() {
			return
		}
		id := strings.TrimSpace(backupID.Text())
		if id == "" {
			status.SetText("ابتدا شناسه بکاپ را از فهرست بکاپ‌ها وارد کنید.")
			return
		}
		answer := qt.QMessageBox_Question2(parent, "حذف بکاپ ابری", "این بکاپ برای همیشه حذف شود؟ این کار قابل بازگشت نیست.", qt.QMessageBox__Yes, qt.QMessageBox__No)
		if answer != int(qt.QMessageBox__Yes) {
			return
		}
		current := client
		run(func() (func(), error) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := current.DeleteBackup(ctx, id); err != nil {
				return nil, err
			}
			return func() {
				status.SetText("بکاپ حذف شد. برای تازه‌سازی فهرست، «مشاهده حساب و فهرست بکاپ‌ها» را بزنید.")
			}, nil
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
	startSync = func(confirmDesktopWins bool) {
		if !requireClient() {
			return
		}
		if confirmDesktopWins {
			answer := qt.QMessageBox_Question2(parent, "همگام‌سازی فضای کار", "تغییرهای مستقل وب و دسکتاپ به‌صورت فیلدی ادغام می‌شوند. اگر یک فیلد در هر دو طرف تغییر کرده باشد، مقدار دسکتاپ انتخاب می‌شود. ادامه می‌دهید؟", qt.QMessageBox__Yes, qt.QMessageBox__No)
			if answer != int(qt.QMessageBox__Yes) {
				return
			}
		}
		current := client
		run(func() (func(), error) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			pushCursor, initialized, err := store.LocalCloudPushCursor(ctx)
			if err != nil {
				return nil, err
			}
			rollbackPath := filepath.Join(filepath.Dir(store.Path()), "hesora-sync-rollback-"+time.Now().Format("20060102-150405.000000000")+".db")
			pushed := 0
			fieldConflicts := 0
			bootstrap := !initialized
			applyCursor := pushCursor
			if !initialized {
				// Capture the log boundary before snapshotting so changes made while
				// the backup is created remain pending for the next incremental push.
				baselineCursor, cursorErr := store.LatestLocalCloudChangeID(ctx)
				if cursorErr != nil {
					return nil, cursorErr
				}
				account, accountErr := current.Account(ctx)
				if accountErr != nil {
					return nil, accountErr
				}
				if err = store.SeedDesktopCloudMappings(ctx, account.User.ID); err != nil {
					return nil, err
				}
				// Seeding templates can enqueue synthetic local events. They belong
				// after the backup boundary, but must be included in the cursor check
				// that protects applying the initial server snapshot.
				applyCursor, cursorErr = store.LatestLocalCloudChangeID(ctx)
				if cursorErr != nil {
					return nil, cursorErr
				}
				if err = store.BackupTo(ctx, rollbackPath); err != nil {
					return nil, err
				}
				backup, uploadErr := current.UploadBackup(ctx, rollbackPath)
				if uploadErr != nil {
					return nil, uploadErr
				}
				if err = current.ImportBackupToWorkspace(ctx, backup.ID); err != nil {
					return nil, err
				}
				pushCursor = baselineCursor
			} else {
				if err = store.BackupTo(ctx, rollbackPath); err != nil {
					return nil, err
				}
				pushCursor, pushed, fieldConflicts, err = pushLocalCloudChanges(ctx, store, current, pushCursor)
				if err != nil {
					return nil, err
				}
				applyCursor = pushCursor
			}
			cursor, err := store.CloudDocumentCursor(ctx)
			if err != nil {
				return nil, err
			}
			docs, nextCursor, err := current.DocumentsSince(ctx, cursor)
			if err != nil {
				return nil, err
			}
			raw, err := json.Marshal(docs)
			if err != nil {
				return nil, err
			}
			var localDocs []sqlite.CloudDocument
			if err = json.Unmarshal(raw, &localDocs); err != nil {
				return nil, err
			}
			changed, err := store.ApplyCloudDocumentsAtLocalCursor(ctx, localDocs, applyCursor)
			if err != nil {
				return nil, err
			}
			if err = store.SaveCloudDocumentCursor(ctx, nextCursor); err != nil {
				return nil, err
			}
			if !initialized {
				if err = store.SaveLocalCloudPushCursor(ctx, pushCursor); err != nil {
					return nil, err
				}
			}
			localPayments, err := store.ExportPayments(ctx)
			if err != nil {
				return nil, err
			}
			for _, payment := range localPayments {
				allocations := make([]cloud.WorkspacePaymentAllocation, 0, len(payment.Allocations))
				for _, a := range payment.Allocations {
					allocations = append(allocations, cloud.WorkspacePaymentAllocation{DocumentID: a.DocumentID, Number: a.Number, Amount: a.Amount})
				}
				_, err = current.SaveWorkspacePayment(ctx, cloud.WorkspacePayment{ID: payment.ID, ExternalRef: payment.ExternalRef, Amount: payment.Amount, Currency: payment.Currency, PaidAt: payment.PaidAt, Method: payment.Method, Note: payment.Note, Allocations: allocations})
				if err != nil {
					return nil, fmt.Errorf("sync payment %s: %w", payment.ExternalRef, err)
				}
			}
			remotePayments, err := current.WorkspacePayments(ctx)
			if err != nil {
				return nil, err
			}
			localRemotePayments := make([]sqlite.SyncedPayment, 0, len(remotePayments))
			for _, payment := range remotePayments {
				allocations := make([]sqlite.SyncedPaymentAllocation, 0, len(payment.Allocations))
				for _, a := range payment.Allocations {
					allocations = append(allocations, sqlite.SyncedPaymentAllocation{DocumentID: a.DocumentID, Number: a.Number, Amount: a.Amount})
				}
				localRemotePayments = append(localRemotePayments, sqlite.SyncedPayment{ID: payment.ID, ExternalRef: payment.ExternalRef, Amount: payment.Amount, Currency: payment.Currency, PaidAt: payment.PaidAt, Method: payment.Method, Note: payment.Note, ReversedAt: payment.ReversedAt, ReversalReason: payment.ReversalReason, Allocations: allocations})
			}
			changedPayments, err := store.ApplyPayments(ctx, localRemotePayments)
			if err != nil {
				return nil, err
			}
			pruneSyncRollbackBackups(filepath.Dir(store.Path()), rollbackPath)
			return func() {
				transfer := fmt.Sprintf("%d changed local documents sent", pushed)
				if bootstrap {
					transfer = "initial desktop snapshot uploaded"
				}
				conflictSummary := ""
				if fieldConflicts > 0 {
					conflictSummary = fmt.Sprintf(" %d conflicts used the desktop value under your selected policy.", fieldConflicts)
				}
				status.SetText(fmt.Sprintf("Cloud sync complete: %s, %d web documents and %d new/reversed receipts applied.%s Local rollback backup: %s", transfer, changed, changedPayments, conflictSummary, filepath.Base(rollbackPath)))
			}, nil
		})
	}
	syncWorkspace.OnClicked(func() { startSync(true) })
	autoSync.OnClicked(func() {
		enabled := autoSync.IsChecked()
		if enabled {
			answer := qt.QMessageBox_Question2(parent, "همگام‌سازی خودکار", "هر ۵ دقیقه سندهای دسکتاپ با وب همگام می‌شوند و در تعارض، نسخهٔ دسکتاپ جایگزین نسخهٔ وب خواهد شد. این سیاست را می‌پذیرید؟", qt.QMessageBox__Yes, qt.QMessageBox__No)
			if answer != int(qt.QMessageBox__Yes) {
				autoSync.SetChecked(false)
				return
			}
		}
		value := "false"
		if enabled {
			value = "true"
		}
		if err := store.SetSetting(context.Background(), domain.Setting{Key: "cloud.auto_sync", Value: value}); err != nil {
			autoSync.SetChecked(!enabled)
			status.SetText("ذخیره تنظیم همگام‌سازی ناموفق بود: " + err.Error())
			return
		}
		if enabled {
			autoSyncTimer.Start2()
			if client != nil {
				startSync(false)
			} else {
				status.SetText("همگام‌سازی خودکار پس از ورود به حساب ابری شروع می‌شود.")
			}
		} else {
			autoSyncTimer.Stop()
			status.SetText("همگام‌سازی خودکار خاموش شد.")
		}
	})
	autoSyncTimer.OnTimeout(func() {
		if autoSync.IsChecked() && client != nil && !busy {
			startSync(false)
		}
	})
	if autoSync.IsChecked() {
		autoSyncTimer.Start2()
	}
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
	if savedLicense, err := files.ReadToken(); err == nil {
		connectWithLicense(savedLicense)
	}
	return connectWithLicense
}

func pushLocalCloudChanges(ctx context.Context, store *sqlite.Store, current *cloud.Client, after int64) (int64, int, int, error) {
	cursor := after
	pushed := 0
	conflictCount := 0
	for {
		pending, nextCursor, more, err := store.PendingLocalCloudChanges(ctx, cursor, 200)
		if err != nil {
			return cursor, pushed, conflictCount, err
		}
		for _, localDoc := range pending {
			remote, conflicts, err := current.PutDocumentThreeWay(ctx, cloud.Document{ID: localDoc.ID, Kind: localDoc.Kind, Version: localDoc.Version, Deleted: localDoc.Deleted, Data: localDoc.Data}, localDoc.BaseData, true)
			if err != nil {
				return cursor, pushed, conflictCount, fmt.Errorf("sync local %s %d: %w", localDoc.Kind, localDoc.LocalID, err)
			}
			conflictCount += len(conflicts)
			localDoc.Data = remote.Data
			if err = store.AcknowledgeLocalCloudDocument(ctx, localDoc, remote.Version); err != nil {
				return cursor, pushed, conflictCount, err
			}
			if err = store.SaveLocalCloudPushCursor(ctx, localDoc.ChangeCursor); err != nil {
				return cursor, pushed, conflictCount, err
			}
			pushed++
		}
		if err = store.SaveLocalCloudPushCursor(ctx, nextCursor); err != nil {
			return cursor, pushed, conflictCount, err
		}
		cursor = nextCursor
		if !more {
			return cursor, pushed, conflictCount, nil
		}
	}
}

func pruneSyncRollbackBackups(dir, keep string) {
	paths, err := filepath.Glob(filepath.Join(dir, "hesora-sync-rollback-*.db"))
	if err != nil {
		return
	}
	for _, path := range paths {
		if path != keep {
			_ = os.Remove(path)
		}
	}
}
